package ressyncrsync

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/drivers/ressync"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/proc"
	"github.com/opensvc/om3/v3/util/schedule"
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
		Schedule       string
		ResetOptions   bool
		Snap           bool
		Snooze         *time.Duration
		Nodes          []string
		DRPNodes       []string
		ObjectID       uuid.UUID
		Timeout        *time.Duration
		Topology       topology.T
	}

	modeT uint
)

const (
	rsync    = "rsync"
	lockName = "sync"

	modeFull modeT = iota
	modeIncr
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Running() (resource.RunningInfoList, error) {
	return t.RunningFromLock(lockName)
}

func (t *T) Full(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSync(ctx, modeFull, target)
}

func (t *T) Update(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	unlock, err := t.Lock(disable, timeout, lockName)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedSync(ctx, modeIncr, target)
}

func (t *T) lockedSync(ctx context.Context, mode modeT, target []string) (err error) {
	if len(target) == 0 {
		target = t.Target
	}

	isCron := actioncontext.IsCron(ctx)

	if t.isFlexAndNotPrimary() {
		t.Log().Errorf("This flex instance is not primary. Only %s can sync", t.Nodes[0])
		return fmt.Errorf("this flex instance is not primary. only %s can sync", t.Nodes[0])
	}

	if v, rids := t.IsInstanceSufficientlyStarted(ctx); !v {
		t.Log().Errorf("The instance is not sufficiently started (%s). Refuse to sync to protect the data of the started remote instance", strings.Join(rids, ","))
		return fmt.Errorf("the instance is not sufficiently started (%s). refuse to sync to protect the data of the started remote instance", strings.Join(rids, ","))
	}
	nodenames := t.GetTargetPeernames(target, t.Nodes, t.DRPNodes)
	if len(nodenames) == 0 {
		t.Log().Infof("no peer to sync")
		return nil
	}
	for _, nodename := range nodenames {
		if err := t.isSendAllowedToPeerEnv(nodename); err != nil {
			if isCron {
				t.Log().Tracef("%s", err)
			} else {
				t.Log().Infof("%s", err)
			}
			continue
		}
		if err := t.peerSync(ctx, mode, nodename); err != nil {
			return err
		}
		if t.WritePeerLastSync(nodename, nodenames); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) Kill(ctx context.Context) error {
	return nil
}

// maxDelay return the configured max_delay if set.
// If not set, return the duration from now to the end of the
// next schedule period.
func (t *T) maxDelay(lastSync time.Time) *time.Duration {
	if t.MaxDelay != nil {
		return t.MaxDelay
	}
	sched := schedule.New(t.Schedule)
	begin, duration, err := sched.Next(schedule.NextWithLast(lastSync))
	if err != nil {
		return nil
	}
	end := begin.Add(duration)
	maxDelay := end.Sub(time.Now())
	if maxDelay < 0 {
		maxDelay = 0
	}
	return &maxDelay
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
	return t.StatusLastSync(nodenames)
}

func (t *T) getTargetNodenames(isSourceNode bool) []string {
	if isSourceNode {
		// if the instance is active, check last sync timestamp for each peer
		return t.GetTargetPeernames(t.Target, t.Nodes, t.DRPNodes)
	} else {
		// if the instance is passive, check last sync timestamp for the local node (received from the source node)
		return []string{hostname.Hostname()}
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
		Action: "sync_update",
		Option: "schedule",
		Base:   "",
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

func (t *T) peerSync(ctx context.Context, mode modeT, nodename string) (err error) {
	if v, err := t.isDstFSMounted(nodename); err != nil {
		return err
	} else if !v {
		t.Log().Errorf("The destination fs %s is not mounted on node %s. Refuse to sync %s to protect parent fs", t.DstFS, nodename, t.Dst)
		return fmt.Errorf("the destination fs %s is not mounted on node %s. refuse to sync %s to protect parent fs", t.DstFS, nodename, t.Dst)
	}
	options := t.fullOptions()
	dst := t.user() + "@" + nodename + ":" + t.Dst
	args := append([]string{}, options...)
	if matches, err := filepath.Glob(t.Src); err != nil {
		return err
	} else {
		args = append(args, matches...)
	}
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
		Infof("sync stat")

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

func (t *T) isSendAllowedToPeerEnv(nodename string) error {
	var localEnv, peerEnv string
	nodesInfo, err := nodesinfo.Load()
	if err != nil {
		return fmt.Errorf("get nodes info: %w", err)
	}
	getEnv := func(n string, s *string) error {
		if m, ok := nodesInfo[n]; !ok {
			return fmt.Errorf("node %s not found in nodes_info.json", n)
		} else {
			*s = m.Env
		}
		return nil
	}
	if err := getEnv(hostname.Hostname(), &localEnv); err != nil {
		return err
	}
	if err := getEnv(nodename, &peerEnv); err != nil {
		return err
	}
	if localEnv != "PRD" && peerEnv == "PRD" {
		return fmt.Errorf("refuse to sync from a non-PRD node to a PRD node")
	}
	return nil
}
