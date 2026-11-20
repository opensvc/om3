package ressyncrsync

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/fssnap"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/proc"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		resource.SSH
		BandwidthLimit string
		Src            string
		Dst            string
		DstFS          string
		User           string
		Options        []string
		Target         []string
		ResetOptions   bool
		Snap           bool
		DelaySnap      bool
		Snooze         *time.Duration
		Nodes          []string
		DRPNodes       []string
		ObjectID       uuid.UUID
		Timeout        *time.Duration
		Topology       topology.T

		// prepared is what PrepareSync took for the transfers of the
		// action, nil when it took nothing: the transfers copy from
		// it, the lock and the run held until its release.
		prepared *preparedSync

		// newSnapper returns the snapshots of an update, replaced by the
		// tests.
		newSnapper func() snapper
	}

	// preparedSync is the snapshots PrepareSync took.
	preparedSync struct {
		// sources are the paths the transfers copy, in the snapshots.
		sources []string
	}

	// snapper takes the snapshots the transfers copy from, and removes
	// them: a *fssnap.Set.
	snapper interface {
		Create(ctx context.Context, paths []string) ([]string, error)
		RemoveDetached() error
	}

	modeT uint
)

const (
	rsync    = "rsync"
	lockName = "sync"

	modeFull modeT = iota
	modeIncr
)

// SyncsTo implements resource.PeerSyncer.
func (t *T) SyncsTo(nodename string) bool {
	return slices.Contains(t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes), nodename)
}

// SyncSource implements resource.PeerSyncer.
func (t *T) SyncSource() string {
	return t.Src
}

func New() resource.Driver {
	return &T{}
}

func (t *T) Full(ctx context.Context) error {
	return t.sync(ctx, modeFull)
}

func (t *T) Update(ctx context.Context) error {
	return t.sync(ctx, modeIncr)
}

// sync runs an update or a full, under the lock of the resource, which a
// PrepareSync of the action holds already.
func (t *T) sync(ctx context.Context, mode modeT) error {
	target := actioncontext.Target(ctx)
	if t.prepared != nil {
		return t.lockedSync(ctx, mode, target)
	}
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSync(ctx, mode, target)
}

// plan returns the peers an update or a full sends to, and the sources it
// copies, no peer when it sends nothing, as the environment, or no peer to
// send to, says, logged unless quiet. An error is a refusal, as a flex
// instance not primary.
func (t *T) plan(ctx context.Context, target []string, quiet bool) ([]string, []string, error) {
	isCron := actioncontext.IsCron(ctx)
	if t.isFlexAndNotPrimary() {
		if !quiet {
			t.Log().Errorf("This flex instance is not primary. Only %s can sync", t.Nodes[0])
		}
		return nil, nil, fmt.Errorf("this flex instance is not primary. only %s can sync", t.Nodes[0])
	}
	if reason := t.EnvSkipReason(); reason != "" {
		switch {
		case quiet:
		case isCron:
			t.Log().Tracef("%s", reason)
		default:
			t.Log().Infof("%s", reason)
		}
		return nil, nil, nil
	}
	if v, reason := t.IsInstanceSufficientlyStarted(ctx); !v {
		return nil, nil, fmt.Errorf("the instance is not sufficiently started (%s). refuse to sync to protect the data of the started remote instance", reason)
	}
	nodenames, err := t.SelectPeernames(target, t.Target, t.Nodes, t.DRPNodes)
	if err != nil {
		return nil, nil, err
	}
	if len(nodenames) == 0 {
		if !quiet {
			t.Log().Infof("no peer to sync")
		}
		return nil, nil, nil
	}
	sources, err := filepath.Glob(t.Src)
	if err != nil {
		return nil, nil, err
	}
	return nodenames, sources, nil
}

// snapper returns the snapshots of an update.
func (t *T) snapper() snapper {
	if t.newSnapper != nil {
		return t.newSnapper()
	}
	return t.snapSet()
}

// CopiesSyncSnapshot implements resource.SyncSnapshotCopier: a resource
// copying a snapshot takes it with the ones of the other sync resources of
// the action, unless delay_snap has it take it just before its own transfer.
func (t *T) CopiesSyncSnapshot() bool {
	return t.Snap && !t.DelaySnap
}

// WillPrepareSync implements resource.SyncPreparer: a resource copying a
// snapshot takes it before the transfers of the action, unless delay_snap
// has it take it just before its own, or it would transfer nothing.
func (t *T) WillPrepareSync(ctx context.Context) bool {
	if !t.CopiesSyncSnapshot() {
		return false
	}
	nodenames, sources, err := t.plan(ctx, actioncontext.Target(ctx), true)
	return err == nil && len(nodenames) > 0 && len(sources) > 0
}

// PrepareSync implements resource.SyncPreparer: it takes the snapshots the
// transfers of the action copy from, under the lock of the resource, held
// with the run until the release, which removes them. A concurrent update
// would remove the snapshots it took, as left by an interrupted run.
func (t *T) PrepareSync(ctx context.Context) (func() error, error) {
	unlock, err := t.Lock(actioncontext.IsLockDisabled(ctx), actioncontext.LockTimeout(ctx), lockName)
	if err != nil {
		return nil, err
	}
	done, err := t.StartRun()
	if err != nil {
		unlock()
		return nil, err
	}
	set := t.snapper()
	release := func() error {
		t.prepared = nil
		defer unlock()
		defer done()
		if err := set.RemoveDetached(); err != nil {
			return fmt.Errorf("remove the snapshots: %w", err)
		}
		return nil
	}
	_, sources, err := t.plan(ctx, actioncontext.Target(ctx), true)
	if err == nil {
		sources, err = set.Create(ctx, sources)
	}
	if err != nil {
		return nil, errors.Join(err, release())
	}
	t.prepared = &preparedSync{sources: sources}
	return release, nil
}

func (t *T) lockedSync(ctx context.Context, mode modeT, target []string) (err error) {
	isCron := actioncontext.IsCron(ctx)
	nodenames, sources, err := t.plan(ctx, target, false)
	if err != nil || len(nodenames) == 0 {
		return err
	}
	if t.prepared != nil {
		// The snapshots the action took before its first transfer.
		sources = t.prepared.sources
	} else {
		done, err := t.StartRun()
		if err != nil {
			return err
		}
		defer done()
	}
	if t.prepared == nil && t.Snap && len(sources) > 0 {
		set := t.snapper()
		// Whatever the outcome, and the snapshots of an interrupted run
		// with them: a snapshot left fills its volume, and a mount left
		// is a mount nobody asked for.
		defer func() {
			if rmErr := set.RemoveDetached(); rmErr != nil {
				err = errors.Join(err, fmt.Errorf("remove the snapshots: %w", rmErr))
			}
		}()
		if sources, err = set.Create(ctx, sources); err != nil {
			return err
		}
	}
	// The peers are synced at once, each on its own: one failing, or slow,
	// does not hold the others back, and all the failures are reported.
	var (
		wg   sync.WaitGroup
		errs = make([]error, len(nodenames))
	)
	for i, nodename := range nodenames {
		if err := t.PeerEnvRefusal(nodename); err != nil {
			if isCron {
				t.Log().Tracef("%s", err)
			} else {
				t.Log().Infof("%s", err)
			}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := t.peerSync(ctx, mode, nodename, sources); err != nil {
				errs[i] = fmt.Errorf("%s: %w", nodename, err)
				return
			}
			if err := t.WritePeerLastSync(ctx, nodename); err != nil {
				errs[i] = fmt.Errorf("%s: write last sync: %w", nodename, err)
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// snapSet is the snapshots an update copies from when snap is set, in the
// var directory of the resource, named after the object and the resource:
// the snapshots of two resources never collide, and the ones an interrupted
// run left are known to the next.
func (t *T) snapSet() *fssnap.Set {
	return &fssnap.Set{
		Dir:           filepath.Join(t.VarDir(), "snap"),
		Name:          "osvc_sync_" + strings.ReplaceAll(t.ObjectID.String(), "-", "")[:8] + "_" + strings.ReplaceAll(t.RID(), "#", "."),
		Log:           t.Log(),
		OneFileSystem: isOneFileSystem(t.fullOptions()),
	}
}

// isOneFileSystem says the rsync options keep the copy on the filesystem of
// each source, as -x, in a group of short options or alone, and
// --one-file-system do.
func isOneFileSystem(options []string) bool {
	for _, o := range options {
		switch {
		case o == "--one-file-system":
			return true
		case strings.HasPrefix(o, "--"):
		case strings.HasPrefix(o, "-") && strings.Contains(o[1:], "x"):
			return true
		}
	}
	return false
}

func (t *T) Kill(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	var isSourceNode bool
	if v, _ := t.IsInstanceSufficientlyStarted(ctx); !v {
		isSourceNode = false
	} else if t.isFlexAndNotPrimary() {
		isSourceNode = false
	} else {
		isSourceNode = true
	}
	nodenames := t.getTargetNodenames(isSourceNode)
	state := t.StatusLastSync(nodenames, !isSourceNode)
	if t.Snap && fssnap.Pending(t.snapSet().Dir) {
		t.StatusLog().Warn("an interrupted update left snapshots, which the next update removes")
		state.Add(status.Warn)
	}
	return state
}

func (t *T) getTargetNodenames(isSourceNode bool) []string {
	if isSourceNode {
		// if the instance is active, check last sync timestamp for each peer
		return t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)
	} else {
		// if the instance is passive, check last sync timestamp for the local node (received from the source node)
		return t.ReceivingNodenames(t.Target, t.Nodes, t.DRPNodes)
	}
}

func (t *T) running(ctx context.Context) bool {
	return false
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	switch {
	case t.Src != "" && len(t.Target) > 0:
		return t.Src + " to " + strings.Join(t.Target, " ")
	case t.Src != "":
		return t.Src + " to void"
	case len(t.Target) > 0:
		return "nothing to " + strings.Join(t.Target, " ")
	default:
		return ""
	}
}

func (t *T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}

func (t *T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action:                   "update",
		Option:                   "schedule",
		Base:                     "",
		RequireReplicationSource: true,
		Require:                  t.UpdateRequires,
		Log:                      t.LogRuns,
	}
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) fullOptions() []string {
	a := args.New()
	if !t.ResetOptions {
		a.Append("-HAXpogDtrlvx", "--stats", "--delete", "--force")
	}
	a.Append(t.Options...)
	if !capabilities.Has(drvID.Cap() + "xattrs") {
		a.DropOption("-X")
	}
	if !capabilities.Has(drvID.Cap() + "acls") {
		a.DropOption("-A")
	}
	if t.Timeout != nil {
		a.DropOption("--timeout")
		a.Append("--timeout=" + fmt.Sprint(int(t.Timeout.Seconds())))
	}
	if sshKeyFile := t.GetSSHKeyFile(); sshKeyFile != "" {
		a.Append("-e", "ssh -i "+sshKeyFile)
	}
	a.Append(t.bandwitdthLimitOptions()...)
	return a.Get()
}

func (t *T) bandwitdthLimitOptions() []string {
	if t.BandwidthLimit != "" {
		return []string{"--bwlimit=" + t.BandwidthLimit}
	} else {
		return []string{}
	}
}

func (t *T) user() string {
	if t.User != "" {
		return t.User
	} else {
		return "root"
	}
}

func (t *T) peerSync(ctx context.Context, mode modeT, nodename string, sources []string) (err error) {
	if v, err := t.isDstFSMounted(nodename); err != nil {
		return err
	} else if !v {
		t.Log().Errorf("The destination fs %s is not mounted on node %s. Refuse to sync %s to protect parent fs", t.DstFS, nodename, t.Dst)
		return fmt.Errorf("the destination fs %s is not mounted on node %s. refuse to sync %s to protect parent fs", t.DstFS, nodename, t.Dst)
	}
	options := t.fullOptions()
	dst := t.user() + "@" + nodename + ":" + t.Dst
	args := append([]string{}, options...)
	args = append(args, sources...)
	args = append(args, dst)
	var timeout time.Duration
	if t.Timeout != nil {
		timeout = *t.Timeout
	}
	addBytesSent := func(line string, stats *ressync.Stats) {
		prefix := "Total bytes sent: "
		prefixLen := len(prefix)
		if !strings.HasPrefix(line, prefix) {
			return
		}

		// strip the comma thousand separator
		line = strings.Replace(line, ",", "", -1)

		if i, err := strconv.ParseUint(line[prefixLen:], 10, 64); err == nil {
			stats.SentBytes = i
		} else {
			t.Log().Warnf("error parsing rsync bytes sent: %s", err)
		}
	}

	addBytesReceived := func(line string, stats *ressync.Stats) {
		prefix := "Total bytes received: "
		prefixLen := len(prefix)
		if !strings.HasPrefix(line, prefix) {
			return
		}

		// strip the comma thousand separator
		line = strings.Replace(line, ",", "", -1)

		if i, err := strconv.ParseUint(line[prefixLen:], 10, 64); err == nil {
			stats.ReceivedBytes = i
		} else {
			t.Log().Warnf("error parsing rsync bytes received: %s", err)
		}
	}

	stats := ressync.NewStats(nodename)

	cmd := command.New(
		command.WithName(rsync),
		command.WithArgs(args),
		command.WithTimeout(timeout),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithOnStdoutLine(func(line string) {
			addBytesSent(line, stats)
			addBytesReceived(line, stats)
		}),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	stats.Close()
	t.Log().
		Attr("speed_bps", stats.SpeedBPS()).
		Attr("duration", stats.Duration()).
		Attr("sent_b", stats.SentBytes).
		Attr("received_b", stats.ReceivedBytes).
		Infof("sync stat: %s: sent %dB received %dB in %s (%.2fB/s)", nodename, stats.SentBytes, stats.ReceivedBytes, stats.Duration(), stats.SpeedBPS())

	return nil
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	target := sort.StringSlice(t.Target)
	sort.Sort(target)
	m := resource.InfoKeys{
		{Key: "src", Value: t.Src},
		{Key: "dst", Value: t.Dst},
		{Key: "bwlimit", Value: t.BandwidthLimit},
		{Key: "snap", Value: fmt.Sprintf("%v", t.Snap)},
		{Key: "target", Value: strings.Join(target, " ")},
		{Key: "options", Value: strings.Join(t.Options, " ")},
		{Key: "reset_options", Value: fmt.Sprintf("%v", t.ResetOptions)},
	}
	if t.Timeout != nil {
		m = append(m, resource.InfoKey{Key: "timeout", Value: fmt.Sprintf("%s", t.Timeout)})
	}
	if t.DstFS != "" {
		m = append(m, resource.InfoKey{Key: "dstfs", Value: fmt.Sprintf("%v", t.DstFS)})
	}
	return m, nil
}

func (t *T) isDstFSMounted(nodename string) (bool, error) {
	if t.DstFS == "" {
		return true, nil
	}
	return t.isFSMounted(nodename, t.DstFS)
}

func (t *T) isFSMounted(nodename, mnt string) (bool, error) {
	user := t.user()
	a := args.New()
	if sshKeyFile := t.GetSSHKeyFile(); sshKeyFile != "" {
		a.Append("-i", sshKeyFile)
	}
	a.Append(user + "@" + nodename)
	a.Append("stat --printf=%m " + mnt)
	cmd := command.New(
		command.WithName("ssh"),
		command.WithArgs(a.Get()),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return false, err
	}
	same := string(b) == mnt
	return same, nil
}

func (t *T) isFlexAndNotPrimary() bool {
	if t.Topology != topology.Flex {
		return false
	}
	if hostname.Hostname() == t.Nodes[0] {
		return false
	}
	return true
}
