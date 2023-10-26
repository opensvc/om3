package ressyncrsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/core/statusbus"
	"github.com/opensvc/om3/core/topology"
	"github.com/opensvc/om3/drivers/ressync"
	"github.com/opensvc/om3/util/args"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/proc"
	"github.com/opensvc/om3/util/progress"
	"github.com/opensvc/om3/util/schedule"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/rs/zerolog"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		Path           naming.Path
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
	rsync = "rsync"

	modeFull modeT = iota
	modeIncr
)

func New() resource.Driver {
	return &T{}
}

func (t T) IsRunning() bool {
	err := t.DoWithLock(false, time.Second*0, "run", func() error {
		return nil
	})
	return err != nil
}

func (t T) Full(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	err := t.DoWithLock(disable, timeout, "sync", func() error {
		return t.lockedSync(ctx, modeFull, target)
	})
	return err
}

func (t T) Update(ctx context.Context) error {
	disable := actioncontext.IsLockDisabled(ctx)
	timeout := actioncontext.LockTimeout(ctx)
	target := actioncontext.Target(ctx)
	err := t.DoWithLock(disable, timeout, "sync", func() error {
		return t.lockedSync(ctx, modeIncr, target)
	})
	return err
}

func (t T) lockedSync(ctx context.Context, mode modeT, target []string) (err error) {
	if len(target) == 0 {
		target = t.Target
	}

	isCron := actioncontext.IsCron(ctx)

	if t.isFlexAndNotPrimary() {
		t.Log().Errorf("This flex instance is not primary. Only %s can sync", t.Nodes[0])
		return fmt.Errorf("this flex instance is not primary. only %s can sync", t.Nodes[0])
	}

	if v, rids := t.isInstanceSufficientlyStarted(ctx); !v {
		t.Log().Errorf("The instance is not sufficiently started (%s). Refuse to sync to protect the data of the started remote instance", strings.Join(rids, ","))
		return fmt.Errorf("the instance is not sufficiently started (%s). refuse to sync to protect the data of the started remote instance", strings.Join(rids, ","))
	}
	for _, nodename := range t.getTargetPeernames(target) {
		if err := t.isSendAllowedToPeerEnv(nodename); err != nil {
			if isCron {
				t.Log().Debugf("%s", err)
			} else {
				t.Log().Infof("%s", err)
			}
			continue
		}
		t.progress(ctx, nodename)
		if err := t.peerSync(ctx, mode, nodename); err != nil {
			return err
		}
		if err := t.writeLastSync(nodename); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) Kill(ctx context.Context) error {
	return nil
}

func (t *T) progress(ctx context.Context, nodename string, more ...any) {
	if view := progress.ViewFromContext(ctx); view != nil {
		key := append(t.ProgressKey(), nodename)
		view.Info(key, more)
	}
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
	if v, _ := t.isInstanceSufficientlyStarted(ctx); !v {
		isSourceNode = false
	} else if t.isFlexAndNotPrimary() {
		isSourceNode = false
	} else {
		isSourceNode = true
	}
	return t.statusLastSync(isSourceNode)
}

func (t T) writeLastSync(nodename string) error {
	p := t.lastSyncFile(nodename)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func (t T) readLastSync(nodename string) (time.Time, error) {
	var tm time.Time
	p := t.lastSyncFile(nodename)
	info, err := os.Stat(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return tm, nil
	case err != nil:
		return tm, err
	default:
		return info.ModTime(), nil
	}
}

func (t T) lastSyncFile(nodename string) string {
	return filepath.Join(t.VarDir(), "last_sync_"+nodename)
}

func (t *T) statusLastSync(isSourceNode bool) status.T {
	state := status.NotApplicable

	var nodenames []string
	if isSourceNode {
		// if the instance is active, check last sync timestamp for each peer
		nodenames = t.getTargetPeernames(t.Target)
	} else {
		// if the instance is passive, check last sync timestamp for the local node (received from the source node)
		nodenames = []string{hostname.Hostname()}
	}

	if len(nodenames) == 0 {
		t.StatusLog().Info("no target nodes")
		return status.NotApplicable
	}

	for _, nodename := range nodenames {
		if tm, err := t.readLastSync(nodename); err != nil {
			t.StatusLog().Error("%s last sync: %s", nodename, err)
		} else if tm.IsZero() {
			t.StatusLog().Warn("%s never synced", nodename)
		} else {
			maxDelay := t.maxDelay(tm)
			if maxDelay == nil {
				t.StatusLog().Info("no schedule and no max delay")
				continue
			}
			elapsed := time.Now().Sub(tm)
			if elapsed > *maxDelay {
				t.StatusLog().Warn("%s last sync at %s (>%s after last)", nodename, tm, maxDelay)
				state.Add(status.Warn)
			} else {
				//t.StatusLog().Info("%s last sync at %s (%s after last)", nodename, tm, maxDelay)
				state.Add(status.Up)
			}
		}
	}
	return state
}

func (t *T) getTargetPeernames(target []string) []string {
	nodenames := make([]string, 0)
	localhost := hostname.Hostname()
	for _, nodename := range t.getTargetNodenames(target) {
		if nodename != localhost {
			nodenames = append(nodenames, nodename)
		}
	}
	return nodenames
}

func (t *T) getTargetNodenames(target []string) []string {
	nodenames := make([]string, 0)
	targetMap := make(map[string]any)
	for _, target := range target {
		targetMap[target] = nil
	}
	if _, ok := targetMap["nodes"]; ok {
		nodenames = append(nodenames, t.Nodes...)
	}
	if _, ok := targetMap["drpnodes"]; ok {
		nodenames = append(nodenames, t.DRPNodes...)
	}
	return nodenames
}

func (t *T) running(ctx context.Context) bool {
	return false
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
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

func (t T) getRunning(cmdArgs []string) (proc.L, error) {
	procs, err := proc.All()
	if err != nil {
		return procs, err
	}
	procs = procs.FilterByEnv("OPENSVC_ID", t.ObjectID.String())
	procs = procs.FilterByEnv("OPENSVC_RID", t.RID())
	return procs, nil
}

func (t T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action: "sync_update",
		Option: "schedule",
		Base:   "",
	}
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) fullOptions() []string {
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
	a.Append(t.bandwitdthLimitOptions()...)
	return a.Get()
}

func (t T) bandwitdthLimitOptions() []string {
	if t.BandwidthLimit != "" {
		return []string{"-bwlimit=" + t.BandwidthLimit}
	} else {
		return []string{}
	}
}

func (t T) user() string {
	if t.User != "" {
		return t.User
	} else {
		return "root"
	}
}

func (t T) peerSync(ctx context.Context, mode modeT, nodename string) (err error) {
	if v, err := t.isDstFSMounted(nodename); err != nil {
		return err
	} else if !v {
		t.Log().Errorf("The destination fs %s is not mounted on node %s. Refuse to sync %s to protect parent fs", t.DstFS, nodename, t.Dst)
		return fmt.Errorf("the destination fs %s is not mounted on node %s. refuse to sync %s to protect parent fs", t.DstFS, nodename, t.Dst)
	}
	options := t.fullOptions()
	dst := t.user() + "@" + nodename + ":" + t.Dst
	args := append([]string{}, options...)
	args = append(args, t.Src, dst)
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

	stats := ressync.NewStats()

	cmd := command.New(
		command.WithName(rsync),
		command.WithArgs(args),
		command.WithTimeout(timeout),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithOnStdoutLine(func(line string) {
			addBytesSent(line, stats)
			addBytesReceived(line, stats)
			rx := fmt.Sprintf("rx:%s", sizeconv.BSizeCompact(float64(stats.ReceivedBytes)))
			tx := fmt.Sprintf("tx:%s", sizeconv.BSizeCompact(float64(stats.SentBytes)))
			t.progress(ctx, nodename, "▶", rx, tx)
		}),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	t.progress(ctx, nodename, rawconfig.Colorize.Optimal("✓"), nil, nil)
	stats.Close()
	logger := plog.Logger{
		Logger: t.Log().With().Float64("speed_bps", stats.SpeedBPS()).Dur("duration", stats.Duration()).Uint64("sent_b", stats.SentBytes).Uint64("received_b", stats.ReceivedBytes).Logger(),
		Prefix: t.Log().Prefix,
	}
	logger.Infof("sync stat")

	if t.peerSyncLastSyncFile(nodename); err != nil {
		return err
	}

	return nil
}

func (t T) peerSyncLastSyncFile(nodename string) error {
	lastSyncFile := t.lastSyncFile(nodename)
	lastSyncFileSrc := t.lastSyncFile(hostname.Hostname())
	schedTimestampFile := filepath.Join(t.GetObjectDriver().VarDir(), "scheduler", "last_sync_update_"+t.RID())
	now := time.Now()
	if err := file.Touch(lastSyncFile, now); err != nil {
		return err
	}
	if err := file.Touch(lastSyncFileSrc, now); err != nil {
		return err
	}
	if err := file.Touch(schedTimestampFile, now); err != nil {
		return err
	}
	dst := t.user() + "@" + nodename + ":/"
	args := make([]string, 0)
	args = append(args, "-R", lastSyncFile, lastSyncFileSrc, schedTimestampFile, dst)
	cmd := command.New(
		command.WithName(rsync),
		command.WithArgs(args),
		command.WithTimeout(10*time.Second),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
	target := sort.StringSlice(t.Target)
	sort.Sort(target)
	m := resource.InfoKeys{
		{"src", t.Src},
		{"dst", t.Dst},
		{"bwlimit", t.BandwidthLimit},
		{"snap", fmt.Sprintf("%v", t.Snap)},
		{"target", strings.Join(target, " ")},
		{"options", strings.Join(t.Options, " ")},
		{"reset_options", fmt.Sprintf("%v", t.ResetOptions)},
	}
	if t.Timeout != nil {
		m = append(m, resource.InfoKey{"timeout", fmt.Sprintf("%s", t.Timeout)})
	}
	if t.DstFS != "" {
		m = append(m, resource.InfoKey{"dstfs", fmt.Sprintf("%v", t.DstFS)})
	}
	return m, nil
}

func (t T) isDstFSMounted(nodename string) (bool, error) {
	if t.DstFS == "" {
		return true, nil
	}
	return isFSMounted(t.user(), nodename, t.DstFS)
}

func isFSMounted(user, nodename, mnt string) (bool, error) {
	a := args.New()
	a.Append(user + "@" + nodename)
	a.Append("stat --printf=%m " + mnt)
	cmd := command.New(
		command.WithName("ssh"),
		command.WithArgs(a.Get()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return false, err
	}
	same := string(b) == mnt
	return same, nil
}

func (t *T) isInstanceSufficientlyStarted(ctx context.Context) (v bool, rids []string) {
	sb := statusbus.FromContext(ctx)
	o := t.GetObjectDriver()
	l := o.ResourcesByDrivergroups([]driver.Group{
		driver.GroupIP,
		driver.GroupFS,
		driver.GroupShare,
		driver.GroupDisk,
		driver.GroupContainer,
	})
	v = true
	for _, r := range l {
		switch r.ID().DriverGroup() {
		case driver.GroupIP:
		case driver.GroupFS:
		case driver.GroupShare:
		case driver.GroupDisk:
			switch r.Manifest().DriverID.Name {
			case "drbd":
				continue
			case "scsireserv":
				continue
			}
		case driver.GroupContainer:
		default:
			continue
		}
		st := sb.Get(r.RID())
		switch st {
		case status.Up:
		case status.StandbyUp:
		case status.NotApplicable:
		default:
			// required resource is not up
			rids = append(rids, r.RID())
			v = false
		}
	}
	return
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
