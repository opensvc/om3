package ressync

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/schedule"
)

type (
	T struct {
		resource.T
		MaxDelay *time.Duration `json:"max_delay"`
		Schedule string         `json:"schedule"`
		Path     naming.Path    `json:"path"`
	}
)

var (
	//go:embed text
	fs embed.FS

	KWMaxDelay = keywords.Keyword{
		Aliases:       []string{"sync_max_delay"},
		Attr:          "MaxDelay",
		Converter:     converters.Duration,
		DefaultOption: "sync_max_delay",
		Option:        "max_delay",
		Text:          keywords.NewText(fs, "text/kw/max_delay"),
	}
	KWSchedule = keywords.Keyword{
		Attr:          "Schedule",
		DefaultOption: "sync_schedule",
		Example:       "00:00-01:00 mon",
		Option:        "schedule",
		Scopable:      true,
		Text:          keywords.NewText(fs, "text/kw/schedule"),
	}

	BaseKeywords = []*keywords.Keyword{
		&KWMaxDelay,
		&KWSchedule,
	}
)

// GetMaxDelay is how long after lastSync the copy is stale: max_delay when
// set, or else derived from the schedule, 0 meaning neither is.
func (t *T) GetMaxDelay(lastSync time.Time) time.Duration {
	if t.MaxDelay != nil {
		return *t.MaxDelay
	}
	return scheduleMaxDelay(t.Schedule, lastSync)
}

// scheduleMaxDelay is how long after lastSync a copy synced on schedule s is
// stale: once the first run due after lastSync is late by half a period of
// the schedule. The run takes time, and the scheduler does not start it on
// the dot, so being due is not being late.
func scheduleMaxDelay(s string, lastSync time.Time) time.Duration {
	if s == "" {
		return 0
	}
	sched := schedule.New(s)
	due, _, err := sched.Next(schedule.NextWithLast(lastSync), schedule.NextWithTime(lastSync))
	if err != nil || due.IsZero() {
		return 0
	}
	after, _, err := sched.Next(schedule.NextWithLast(due), schedule.NextWithTime(due.Add(time.Second)))
	if err != nil || !after.After(due) {
		return due.Sub(lastSync)
	}
	return due.Sub(lastSync) + after.Sub(due)/2
}

func (t *T) StatusLastSync(nodenames []string) status.T {
	state := status.NotApplicable

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
			maxDelay := t.GetMaxDelay(tm)
			if maxDelay == 0 {
				t.StatusLog().Info("no schedule and no max delay")
				continue
			}
			age := time.Since(tm)
			if age > maxDelay {
				t.StatusLog().Warn("%s last sync is too old, at %s (>%s ago)", nodename, tm, maxDelay)
				state.Add(status.Warn)
			} else {
				state.Add(status.Up)
			}
		}
	}
	return state
}

func (t *T) WritePeerLastSync(ctx context.Context, peer string, peers []string) error {
	head := t.GetObjectDriver().VarDir()
	lastSyncFile := t.lastSyncFile(peer)
	lastSyncFileSrc := t.lastSyncFile(hostname.Hostname())
	schedTimestampFile := filepath.Join(head, "scheduler", "last_sync_update_"+t.RID())
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

	c, err := client.New()
	if err != nil {
		return err
	}

	send := func(filename, nodename string) error {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		response, err := c.PostInstanceStateFileWithBody(ctx, nodename, t.Path.Namespace, t.Path.Kind, t.Path.Name, "application/octet-stream", file, func(ctx context.Context, req *http.Request) error {
			req.Header.Add(api.HeaderRelativePath, filename[len(head):])
			return nil
		})
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected response: %s", response.Status)
		}
		return nil
	}

	var errs error

	for _, nodename := range peers {
		for _, filename := range []string{lastSyncFile, lastSyncFileSrc, schedTimestampFile} {
			if err := send(filename, nodename); err != nil {
				errs = errors.Join(errs, fmt.Errorf("failed to send state file %s to node %s: %w", filename, nodename, err))
			}
			t.Log().Infof("state file %s sent to node %s", filename, nodename)
		}
	}

	return errs
}

func (t *T) WriteLastSync(nodename string) error {
	p := t.lastSyncFile(nodename)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func (t *T) readLastSync(nodename string) (time.Time, error) {
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

func (t *T) lastSyncFile(nodename string) string {
	return filepath.Join(t.VarDir(), "last_sync_"+nodename)
}

// expandNodeSelector is the nodes a node selector expression selects.
var expandNodeSelector = func(s string) ([]string, error) {
	return nodeselector.New(s).Expand()
}

// SelectPeernames is the peers a run asked to sync to target reaches, among
// the peers of configured, the target of the configuration. An empty target
// asks all of them. A target is "nodes", "drpnodes", "local", or a node
// selector expression, so a peer that needs syncing on its own is named.
func (t *T) SelectPeernames(target, configured, nodes, drpNodes []string) ([]string, error) {
	peers := t.GetTargetPeernames(configured, nodes, drpNodes)
	if len(target) == 0 {
		return peers, nil
	}
	selected := make([]string, 0, len(peers))
	add := func(nodename string) bool {
		if !slices.Contains(peers, nodename) {
			return false
		}
		if !slices.Contains(selected, nodename) {
			selected = append(selected, nodename)
		}
		return true
	}
	for _, v := range target {
		switch v {
		case "nodes", "drpnodes", "local":
			for _, nodename := range t.GetTargetPeernames([]string{v}, nodes, drpNodes) {
				add(nodename)
			}
			continue
		}
		if add(v) {
			continue
		}
		matched, err := expandNodeSelector(v)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", v, err)
		}
		found := false
		for _, nodename := range matched {
			if add(nodename) {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("target %s selects no peer this resource syncs to (%s)", v, strings.Join(peers, " "))
		}
	}
	return selected, nil
}

func (t *T) GetTargetPeernames(target, nodes, drpNodes []string) []string {
	nodenames := make([]string, 0)
	localhost := hostname.Hostname()
	withLocal := slices.Contains(target, "local")
	for _, nodename := range t.GetTargetNodenames(target, nodes, drpNodes) {
		if nodename != localhost {
			nodenames = append(nodenames, nodename)
		} else if withLocal {
			nodenames = append(nodenames, nodename)
		}
	}
	return nodenames
}

func (t *T) GetTargetNodenames(target, nodes, drpNodes []string) []string {
	nodenames := make([]string, 0)
	targetMap := make(map[string]bool)
	for _, t := range target {
		targetMap[t] = false
	}
	if done, ok := targetMap["local"]; ok && !done {
		nodenames = append(nodenames, hostname.Hostname())
		targetMap["local"] = true
	}
	if done, ok := targetMap["nodes"]; ok && !done {
		nodenames = append(nodenames, nodes...)
		targetMap["nodes"] = true
	}
	if done, ok := targetMap["drpnodes"]; ok && !done {
		nodenames = append(nodenames, drpNodes...)
		targetMap["drpnodes"] = true
	}
	return nodenames
}

// IsInstanceSufficientlyStarted reports whether this node holds the instance
// the data is replicated from, and why not when it does not.
//
// It is the instance whose reference resources are up: the resources holding
// what the data lives on or is reached by, that is every resource but the
// app, sync and task ones, and the disk.scsireserv and disk.drbd ones, which
// are up on the passive nodes too. As v2 did, their aggregated availability
// must be up, or not applicable with the overall status up.
//
// A node where they are not up is a passive one, which must not send: its
// copy would replace the one of the active node. An object with no reference
// resource at all gives no way to tell the active node from the others, so no
// node sends. --force lets a node whose reference resources are neither down
// nor not applicable send anyway, as for a warn status.
func (t *T) IsInstanceSufficientlyStarted(ctx context.Context) (bool, string) {
	sb := statusbus.FromContext(ctx)
	o := t.GetObjectDriver()
	l := o.ResourcesByDrivergroups([]driver.Group{
		driver.GroupIP,
		driver.GroupVolume,
		driver.GroupFS,
		driver.GroupShare,
		driver.GroupDisk,
		driver.GroupContainer,
	})
	refs := make([]refStatus, 0, len(l))
	for _, r := range l {
		if r.ID().DriverGroup() == driver.GroupDisk {
			switch r.DriverID().Name {
			case "drbd", "scsireserv":
				continue
			}
		}
		if r.IsDisabled() {
			continue
		}
		refs = append(refs, refStatus{rid: r.RID(), status: sb.Get(r.RID()), optional: r.IsOptional()})
	}
	ok, reason, forced := isSource(refs, actioncontext.IsForce(ctx))
	if forced {
		t.Log().Infof("sync allowed by --force: %s", reason)
		return true, ""
	}
	return ok, reason
}

type refStatus struct {
	rid      string
	status   status.T
	optional bool
}

// isSource decides from the status of the reference resources whether this
// node holds the active instance. forced says it does only because force is
// set, reason saying why it would not otherwise.
func isSource(refs []refStatus, force bool) (ok bool, reason string, forced bool) {
	avail, overall := status.Undef, status.Undef
	var notUp []string
	for _, r := range refs {
		overall.Add(r.status)
		if !r.optional {
			avail.Add(r.status)
		}
		if r.status != status.Up {
			notUp = append(notUp, fmt.Sprintf("%s:%s", r.rid, r.status))
		}
	}
	if avail == status.StandbyUpWithUp {
		avail = status.Up
	}
	if overall == status.StandbyUpWithUp {
		overall = status.Up
	}
	switch {
	case avail == status.Up:
		return true, "", false
	case avail.Is(status.Undef, status.NotApplicable) && overall == status.Up:
		return true, "", false
	case overall == status.Undef:
		return false, "no ip, volume, fs, share, disk or container resource tells the active instance", false
	}
	reason = fmt.Sprintf("reference resources %s/%s: %s", avail, overall, strings.Join(notUp, ","))
	if force && !avail.Is(status.Down, status.NotApplicable, status.Undef) && !overall.Is(status.Down, status.NotApplicable, status.Undef) {
		return true, reason, true
	}
	return false, reason, false
}
