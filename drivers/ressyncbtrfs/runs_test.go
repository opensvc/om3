package ressyncbtrfs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/btrfs"
)

// fakeSubvol is a subvolume of a fake node.
type fakeSubvol struct {
	uuid     string
	received string
	cgen     int64
	readOnly bool

	// content is what the subvolume holds, a snapshot holding what its
	// origin held when it was taken.
	content string
}

// fakeCluster holds the subvolumes of a btrfs filesystem per node, and sends
// between them the way btrfs would: a received snapshot has the identity of
// the one sent, and a receive from a parent fails unless the receiver holds a
// snapshot of the identity of that parent.
type fakeCluster struct {
	t *T

	// nodes are the subvolumes of each node by path, the local node
	// being "".
	nodes map[string]map[string]*fakeSubvol

	// mounted are the nodes the destination subvolume is mounted on.
	mounted map[string]bool

	// down are the nodes the operations fail on.
	down map[string]bool

	gen   int64
	uuid  int
	sends []string

	// state is the state the local node saved, which the var dir of a
	// resource would hold. It moves with the local node on a failover.
	state  syncState
	states map[string]syncState
}

func newFakeCluster(t *T) *fakeCluster {
	c := &fakeCluster{
		t:       t,
		nodes:   map[string]map[string]*fakeSubvol{},
		mounted: map[string]bool{},
		down:    map[string]bool{},
	}
	t.ops = c
	return c
}

func (c *fakeCluster) node(nodename string) map[string]*fakeSubvol {
	m, ok := c.nodes[nodename]
	if !ok {
		m = map[string]*fakeSubvol{}
		c.nodes[nodename] = m
	}
	return m
}

func (c *fakeCluster) newSubvol(content string, readOnly bool, received string) *fakeSubvol {
	c.gen++
	c.uuid++
	return &fakeSubvol{uuid: fmt.Sprintf("u%d", c.uuid), cgen: c.gen, readOnly: readOnly, received: received, content: content}
}

func (s *fakeSubvol) identity() string {
	if s.received != "" {
		return s.received
	}
	return s.uuid
}

// write sets what the head subvolume of nodename holds.
func (c *fakeCluster) write(nodename, content string) {
	m := c.node(nodename)
	if h, ok := m[c.t.src.subvol]; ok {
		h.content = content
		return
	}
	m[c.t.src.subvol] = c.newSubvol(content, false, "")
}

func (c *fakeCluster) head(nodename string) string {
	if h, ok := c.node(nodename)[c.t.dst.subvol]; ok {
		return h.content
	}
	return ""
}

func (c *fakeCluster) subvols(nodename string, readOnly bool) []btrfs.Subvol {
	l := make([]btrfs.Subvol, 0)
	for p, s := range c.node(nodename) {
		if readOnly && !s.readOnly {
			continue
		}
		l = append(l, btrfs.Subvol{ID: s.cgen, CGen: s.cgen, Path: p, UUID: s.uuid, ReceivedUUID: s.received})
	}
	sort.Slice(l, func(i, j int) bool { return l[i].Path < l[j].Path })
	return l
}

func (c *fakeCluster) listRuns(nodename, label, dir string) ([]run, error) {
	if c.down[nodename] {
		return nil, fmt.Errorf("%s is down", nodename)
	}
	return runsOf(c.subvols(nodename, true), dir)
}

func (c *fakeCluster) nestedSubvols(label, head string) ([]string, error) {
	return nestedOf(c.subvols("", false), c.subvols("", true), head), nil
}

func (c *fakeCluster) takeRun(label, head string, rels []string, dir string) (run, error) {
	m := c.node("")
	for _, rel := range append([]string{""}, rels...) {
		src, ok := m[filepath.Join(head, rel)]
		if !ok {
			return run{}, fmt.Errorf("no subvolume %s", filepath.Join(head, rel))
		}
		m[filepath.Join(dir, snapName(rel))] = c.newSubvol(src.content, true, "")
	}
	runs, err := c.listRuns("", label, filepath.Dir(dir))
	if err != nil {
		return run{}, err
	}
	for _, r := range runs {
		if r.Name == filepath.Base(dir) {
			return r, nil
		}
	}
	return run{}, fmt.Errorf("run %s not found", dir)
}

func (c *fakeCluster) send(ctx context.Context, nodename, srcLabel, path, parent, dstLabel, dir string) error {
	if c.down[nodename] {
		return fmt.Errorf("%s is down", nodename)
	}
	src := c.node("")[path]
	if src == nil || !src.readOnly {
		return fmt.Errorf("send %s: not a read-only snapshot", path)
	}
	dst := c.node(nodename)
	if parent != "" {
		p := c.node("")[parent]
		if p == nil {
			return fmt.Errorf("send: no parent %s", parent)
		}
		found := false
		for _, s := range dst {
			if s.readOnly && (s.received == p.identity() || s.uuid == p.identity()) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("receive: cannot find parent subvolume %s", p.identity())
		}
	}
	c.sends = append(c.sends, fmt.Sprintf("%s %s parent=%s", nodename, path, parent))
	dst[filepath.Join(dir, filepath.Base(path))] = c.newSubvol(src.content, true, src.identity())
	return nil
}

func (c *fakeCluster) installHead(nodename, label, dir string, r run, head string) error {
	if c.mounted[nodename] {
		return fmt.Errorf("subvolume %s is mounted: the service runs there", head)
	}
	m := c.node(nodename)
	s, ok := m[filepath.Join(dir, snapName(""))]
	if !ok {
		return fmt.Errorf("no head snapshot in %s", dir)
	}
	m[head] = c.newSubvol(s.content, false, "")
	return nil
}

func (c *fakeCluster) deleteRuns(nodename, label, dir string, keep []string) error {
	m := c.node(nodename)
	for p := range m {
		rest, ok := strings.CutPrefix(p, dir+"/")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, "/")
		if !slices.Contains(keep, name) {
			delete(m, p)
		}
	}
	return nil
}

func (c *fakeCluster) release() {}

// runNames are the names of the runs of nodename, oldest first.
func (c *fakeCluster) runNames(nodename string) []string {
	runs, _ := c.listRuns(nodename, "", c.t.runsDir())
	l := make([]string, len(runs))
	for i, r := range runs {
		l[i] = r.Name
	}
	return l
}

// failover makes nodename the local node, and the local node a peer named
// formerLocal.
func (c *fakeCluster) failover(nodename, formerLocal string) {
	c.nodes[formerLocal], c.nodes[nodename], c.nodes[""] = c.nodes[""], nil, c.nodes[nodename]
	delete(c.nodes, nodename)
	if c.states == nil {
		c.states = map[string]syncState{}
	}
	c.states[formerLocal] = c.state
	c.state = c.states[nodename]
}

func newTestT(t *testing.T) *T {
	o := &T{Src: "fs1:data", Dst: "fs1:data", Target: []string{"nodes"}}
	require.NoError(t, o.SetRID("sync#1"))
	p, err := naming.ParsePath("dev/svc/s1")
	require.NoError(t, err)
	o.Path = p
	require.NoError(t, o.parse())
	return o
}

// sync runs a sync of the local node to the peers, as lockedSync does once
// the instance is checked started, at the time now.
func (c *fakeCluster) sync(mode modeT, now time.Time, peers ...string) error {
	t := c.t
	state := c.state
	if state.Peers == nil {
		state.Peers = make(peerStates)
	}
	l, err := c.listRuns("", t.src.label, t.runsDir())
	if err != nil {
		return err
	}
	previous := completeRuns(l)
	var newest string
	if len(previous) > 0 {
		newest = previous[len(previous)-1].headID()
	}
	states := state.validPeers(newest)
	var rels []string
	if t.Recursive {
		rels, _ = c.nestedSubvols(t.src.label, t.src.subvol)
	}
	r, err := c.takeRun(t.src.label, t.src.subvol, rels, filepath.Join(t.runsDir(), newRunName(now)))
	if err != nil {
		return err
	}
	local := append(previous, r)
	var errs error
	for _, nodename := range peers {
		st := states[nodename]
		errs = errors.Join(errs, t.syncPeer(context.Background(), mode, nodename, local, r, &st, now))
		states[nodename] = st
	}
	c.state = syncState{LastRunID: r.headID(), Peers: states}
	return errors.Join(errs, t.pruneLocal(local, peers, states))
}

func TestInitialThenIncremental(t *testing.T) {
	o := newTestT(t)
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	c.write("", "v1")
	require.NoError(t, c.sync(modeIncr, now, "b"))
	assert.Equal(t, "v1", c.head("b"), "the peer holds the data")
	require.Len(t, c.sends, 1)
	assert.Contains(t, c.sends[0], "parent=", "a peer with no run is sent the run whole")
	assert.True(t, strings.HasSuffix(c.sends[0], "parent="))

	c.write("", "v2")
	require.NoError(t, c.sync(modeIncr, now.Add(time.Minute), "b"))
	assert.Equal(t, "v2", c.head("b"))
	require.Len(t, c.sends, 2)
	assert.NotContains(t, c.sends[1], "parent= ", "the second run is sent from the first")
	assert.False(t, strings.HasSuffix(c.sends[1], "parent="))

	assert.Equal(t, []string{newRunName(now.Add(time.Minute))}, c.runNames(""), "the source keeps the newest run, the base of the peer")
	assert.Equal(t, []string{newRunName(now.Add(time.Minute))}, c.runNames("b"), "the peer keeps the run it was sent")
}

// TestFailoverAndFailbackWithWritesInBetween is the case the driver is
// written for: each node takes writes while it is the source, and each
// sync after a failover is incremental, from the last run the two hold.
func TestFailoverAndFailbackWithWritesInBetween(t *testing.T) {
	o := newTestT(t)
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	c.write("", "a1")
	require.NoError(t, c.sync(modeIncr, now, "b"))
	c.write("", "a2")
	require.NoError(t, c.sync(modeIncr, now.Add(time.Minute), "b"))
	assert.Equal(t, "a2", c.head("b"))

	// Failover to b, which takes writes, and syncs a.
	c.failover("b", "a")
	assert.Equal(t, "a2", c.head(""), "b starts on the data it was sent")
	c.write("", "b1")
	sends := len(c.sends)
	require.NoError(t, c.sync(modeIncr, now.Add(2*time.Minute), "a"))
	assert.Equal(t, "b1", c.head("a"), "the old source holds the writes of the new one")
	require.Len(t, c.sends, sends+1)
	assert.False(t, strings.HasSuffix(c.sends[sends], "parent="), "the failover sync is incremental")

	// Failback to a, which takes writes, and syncs b.
	c.failover("a", "b")
	assert.Equal(t, "b1", c.head(""))
	c.write("", "a3")
	sends = len(c.sends)
	require.NoError(t, c.sync(modeIncr, now.Add(3*time.Minute), "b"))
	assert.Equal(t, "a3", c.head("b"))
	require.Len(t, c.sends, sends+1)
	assert.False(t, strings.HasSuffix(c.sends[sends], "parent="), "the failback sync is incremental")
}

// TestAFailedSendKeepsTheBaseOfThePeer pins that a peer missing runs is sent
// the larger delta from the base the source kept for it.
func TestAFailedSendKeepsTheBaseOfThePeer(t *testing.T) {
	o := newTestT(t)
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	c.write("", "v1")
	require.NoError(t, c.sync(modeIncr, now, "b", "c"))
	c.down["c"] = true
	c.write("", "v2")
	require.Error(t, c.sync(modeIncr, now.Add(time.Minute), "b", "c"))
	assert.Len(t, c.runNames(""), 2, "the base of c is kept")
	c.down["c"] = false
	c.write("", "v3")
	require.NoError(t, c.sync(modeIncr, now.Add(2*time.Minute), "b", "c"))
	assert.Equal(t, "v3", c.head("c"))
	assert.Equal(t, "v3", c.head("b"))
	assert.Len(t, c.runNames(""), 1)
}

// TestAPeerLaggingTooLongIsStranded pins that a peer missing runs for longer
// than max_lag_age has its base released, and is sent nothing until a full
// copy is asked.
func TestAPeerLaggingTooLongIsStranded(t *testing.T) {
	o := newTestT(t)
	maxLag := time.Hour
	o.MaxLagAge = &maxLag
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	c.write("", "v1")
	require.NoError(t, c.sync(modeIncr, now, "b"))
	c.down["b"] = true
	require.Error(t, c.sync(modeIncr, now.Add(time.Minute), "b"))
	c.down["b"] = false
	err := c.sync(modeIncr, now.Add(2*time.Hour), "b")
	require.ErrorIs(t, err, errStranded)
	assert.Contains(t, err.Error(), "instance full")
	assert.Len(t, c.runNames(""), 1, "the base of the stranded peer is released")

	require.NoError(t, c.sync(modeFull, now.Add(3*time.Hour), "b"))
	c.write("", "v4")
	require.NoError(t, c.sync(modeIncr, now.Add(4*time.Hour), "b"))
	assert.Equal(t, "v4", c.head("b"), "synced again after the full copy")
}

// TestAPeerWithNoRunInCommonIsStranded pins that a peer holding runs, none of
// which the source holds, is not replaced without an administrator asking.
func TestAPeerWithNoRunInCommonIsStranded(t *testing.T) {
	o := newTestT(t)
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	m := c.node("b")
	m[filepath.Join(o.runsDir(), "20200101T000000.000000Z", "_")] = c.newSubvol("old", true, "foreign")
	c.write("", "v1")
	err := c.sync(modeIncr, now, "b")
	require.ErrorIs(t, err, errStranded)
	assert.Empty(t, c.head("b"), "the peer is not replaced")
}

// TestAMountedPeerIsNotReplaced pins that the data of a peer running the
// service is not replaced.
func TestAMountedPeerIsNotReplaced(t *testing.T) {
	o := newTestT(t)
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	c.write("", "v1")
	c.write("b", "b's own")
	c.mounted["b"] = true
	err := c.sync(modeIncr, now, "b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mounted")
	assert.Equal(t, "b's own", c.head("b"))
}

// TestARunReceivedButNotInstalledIsInstalled pins that a sync stopping
// between the receive and the install is completed by the next one.
func TestARunReceivedButNotInstalledIsInstalled(t *testing.T) {
	o := newTestT(t)
	c := newFakeCluster(o)
	now := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)

	c.write("", "v1")
	c.mounted["b"] = true
	require.Error(t, c.sync(modeIncr, now, "b"))
	c.mounted["b"] = false
	c.write("", "v2")
	require.NoError(t, c.sync(modeIncr, now.Add(time.Minute), "b"))
	assert.Equal(t, "v2", c.head("b"))
}

func TestSnapNameRoundTrips(t *testing.T) {
	for _, rel := range []string{"", "a", "a/b", "with space", "per%cent"} {
		got, err := relOfSnapName(snapName(rel))
		require.NoError(t, err)
		assert.Equal(t, rel, got)
	}
	assert.Equal(t, "_", snapName(""))
	assert.Equal(t, "_a%2Fb", snapName("a/b"))
}

func TestRunsOfIgnoresWhatIsNotASnapshotOfARun(t *testing.T) {
	dir := ".osync/dev/svc/s1/sync#1"
	l := []btrfs.Subvol{
		{ID: 1, CGen: 1, Path: "data", UUID: "h"},
		{ID: 2, CGen: 5, Path: dir + "/r2/_", UUID: "u2"},
		{ID: 3, CGen: 3, Path: dir + "/r1/_", UUID: "u1", ReceivedUUID: "x1"},
		{ID: 4, CGen: 4, Path: dir + "/r1/_a", UUID: "u3"},
		{ID: 5, CGen: 6, Path: dir + "/r2/notasnap", UUID: "u4"},
	}
	runs, err := runsOf(l, dir)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, "r1", runs[0].Name, "ordered by the creation of their head snapshot")
	assert.Equal(t, "x1", runs[0].headID(), "a received snapshot is identified by its received uuid")
	assert.Equal(t, []string{"", "a"}, runs[0].rels())
	assert.Equal(t, "r2", runs[1].Name)
}

func TestNestedOfSkipsTheReadOnlySubvolumes(t *testing.T) {
	all := []btrfs.Subvol{
		{ID: 1, Path: "data"},
		{ID: 2, Path: "data/b/c"},
		{ID: 3, Path: "data/a"},
		{ID: 4, Path: "data/.snap/2026-10-09T10:00:00.000000Z"},
		{ID: 5, Path: "datab"},
	}
	ro := []btrfs.Subvol{{ID: 4, Path: "data/.snap/2026-10-09T10:00:00.000000Z"}}
	assert.Equal(t, []string{"a", "b/c"}, nestedOf(all, ro, "data"))
}

func TestDeleteOrderIsDeepestFirst(t *testing.T) {
	l := []btrfs.Subvol{
		{ID: 1, Path: "data"},
		{ID: 2, Path: "data/a"},
		{ID: 3, Path: "data/a/b"},
		{ID: 4, Path: "datab"},
	}
	assert.Equal(t, []string{"data/a/b", "data/a", "data"}, deleteOrder(l, "data"))
}

func TestParseLocationRefusesWhatTheSyncManages(t *testing.T) {
	for _, s := range []string{"fs1", "fs1:", "fs1:/", ":data", "fs1:.osync", "fs1:.osync/x", "fs1:data.osync-new"} {
		_, err := parseLocation(s)
		assert.Errorf(t, err, "%q", s)
	}
	l, err := parseLocation("fs1:/a/b/")
	require.NoError(t, err)
	assert.Equal(t, location{label: "fs1", subvol: "a/b"}, l)
}

func TestProgressWriterRoutesTheProgressLines(t *testing.T) {
	var info, errs strings.Builder
	w := &progressWriter{info: &info, err: &errs}
	_, err := w.Write([]byte("At subvol _\nERROR: cannot find par"))
	require.NoError(t, err)
	_, err = w.Write([]byte("ent subvolume\nAt snapshot _a\n"))
	require.NoError(t, err)
	assert.Equal(t, "At subvol _\nAt snapshot _a\n", info.String())
	assert.Equal(t, "ERROR: cannot find parent subvolume\n", errs.String())
}
