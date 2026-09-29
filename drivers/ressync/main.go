package ressync

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
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

// MaxDelayOrigin says where GetMaxDelay takes its value from, for the
// messages to say why a copy is stale.
func (t *T) MaxDelayOrigin() string {
	if t.MaxDelay != nil {
		return "max_delay"
	}
	return fmt.Sprintf("schedule %s, half a period after the sync due", t.Schedule)
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

// neverSyncedRPOBreach is the RPO breach time of a copy never received: no
// time the node took over from would lose less than everything.
var neverSyncedRPOBreach = time.Unix(0, 0)

// StatusLastSync judges the freshness of the copies of nodenames. receiving
// says this node is one of them, a node the data is replicated to, whose copy
// has a recovery point objective: its max delay.
func (t *T) StatusLastSync(nodenames []string, receiving bool) status.T {
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
			if receiving {
				t.StatusLog().RPOBreachesAt(neverSyncedRPOBreach)
			}
		} else {
			maxDelay := t.GetMaxDelay(tm)
			if maxDelay == 0 {
				t.StatusLog().Info("no schedule and no max delay")
				continue
			}
			// The copy goes stale then, with no event to tell.
			t.StatusLog().ChangesAt(tm.Add(maxDelay))
			if receiving {
				t.StatusLog().RPOBreachesAt(tm.Add(maxDelay))
			}
			age := time.Since(tm)
			if age > maxDelay {
				t.StatusLog().Warn("%s last sync is too old, at %s, more than %s ago (%s)", nodename, tm, maxDelay, t.MaxDelayOrigin())
				state.Add(status.Warn)
			} else {
				state.Add(status.Up)
			}
		}
	}
	return state
}

// WritePeerLastSync records that peer was synced now, and sends it the
// records it reads: its own last sync, which its status tells the freshness of
// its copy from, the last sync of this node, and the last run of the schedule,
// which keeps its scheduler from syncing again too soon if it becomes the
// source.
//
// The records of the other peers are not sent: a node reads them only once it
// is the source, and its first sync writes them.
func (t *T) WritePeerLastSync(ctx context.Context, peer string) error {
	head := t.GetObjectDriver().VarDir()
	lastSyncFile := t.lastSyncFile(peer)
	lastSyncFileSrc := t.lastSyncFile(hostname.Hostname())
	// The file the scheduler keeps the last run of the update schedule of
	// the resource in, named as core/object names it for a resource
	// schedule with no base.
	schedTimestampFile := filepath.Join(head, "scheduler", "last_"+t.RID())
	now := time.Now()
	for _, filename := range []string{lastSyncFile, lastSyncFileSrc, schedTimestampFile} {
		if err := file.Touch(filename, now); err != nil {
			return err
		}
	}
	if peer == hostname.Hostname() {
		return nil
	}

	c, err := client.New()
	if err != nil {
		return err
	}

	send := func(filename string) error {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		response, err := c.PostInstanceStateFileWithBody(ctx, peer, t.Path.Namespace, t.Path.Kind, t.Path.Name, "application/octet-stream", file, func(ctx context.Context, req *http.Request) error {
			req.Header.Add(api.HeaderRelativePath, filename[len(head):])
			return nil
		})
		if err != nil {
			return err
		}
		defer drainClose(response.Body)
		if response.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected response: %s", response.Status)
		}
		return nil
	}

	var errs error
	for _, filename := range []string{lastSyncFile, lastSyncFileSrc, schedTimestampFile} {
		if err := send(filename); err != nil {
			errs = errors.Join(errs, fmt.Errorf("send state file %s to node %s: %w", filename, peer, err))
			continue
		}
		t.Log().Debugf("state file %s sent to node %s", filename, peer)
	}
	return errs
}

// drainClose reads what is left of a response body before closing it, so the
// connection is reused for the next request.
func drainClose(rc io.ReadCloser) {
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
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
// the data is replicated from, and why not when it does not. See
// instance.IsReplicationSource for the rules.
//
// A node where its reference resources are not up is a passive one, which
// must not send: its copy would replace the one of the active node.
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
	refs := make([]instance.ReferenceStatus, 0, len(l))
	for _, r := range l {
		if r.IsDisabled() || !instance.IsReferenceResource(r.ID().DriverGroup(), r.DriverID().Name) {
			continue
		}
		refs = append(refs, instance.ReferenceStatus{RID: r.RID(), Status: sb.Get(r.RID()), Optional: r.IsOptional()})
	}
	ok, reason, forced := instance.IsReplicationSource(refs, actioncontext.IsForce(ctx))
	if forced {
		t.Log().Infof("sync allowed by --force: %s", reason)
		return true, ""
	}
	return ok, reason
}

// DatasetReplicator is a sync resource replicating zfs datasets to its peers,
// as sync.zfs does. It tells the snapshot resources of the object which of
// their datasets reach the peers, and how fresh the replicas are expected to
// be.
type DatasetReplicator interface {
	RID() string
	ReplicatesDataset(dataset string) bool
	GetMaxDelay(lastSync time.Time) time.Duration
	MaxDelayOrigin() string
}
