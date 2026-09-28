package ressynczfs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// fakeCluster holds the snapshots of the dataset of the source and of its
// peers, and sends between them the way zfs would.
type fakeCluster struct {
	t *T

	// snaps are the snapshots of each node, the source being "".
	snaps map[string][]snapshot

	// down are the nodes a send or a list fails on.
	down map[string]bool

	txg     uint64
	guid    int
	held    int64
	avail   int64
	sent    map[string][]string
	pending []string
}

func newFakeCluster(t *T) *fakeCluster {
	c := &fakeCluster{
		t:     t,
		snaps: map[string][]snapshot{},
		down:  map[string]bool{},
		sent:  map[string][]string{},
		avail: 1 << 40,
	}
	t.ops = c
	return c
}

func (c *fakeCluster) newSnapshot(name string) snapshot {
	c.txg++
	c.guid++
	return snapshot{Name: name, GUID: fmt.Sprintf("g%d", c.guid), CreateTxg: c.txg}
}

func (c *fakeCluster) node(nodename string) string {
	if nodename == "src" {
		return ""
	}
	return nodename
}

func (c *fakeCluster) listSnapshots(nodename, dataset string) ([]snapshot, error) {
	nodename = c.node(nodename)
	if c.down[nodename] {
		return nil, fmt.Errorf("%s is down", nodename)
	}
	return append([]snapshot{}, c.snaps[nodename]...), nil
}

func (c *fakeCluster) takeSnapshot(name string) error {
	c.snaps[""] = append(c.snaps[""], c.newSnapshot(name))
	return nil
}

func (c *fakeCluster) destroySnapshot(nodename, name string) error {
	nodename = c.node(nodename)
	l := c.snaps[nodename][:0]
	for _, s := range c.snaps[nodename] {
		if s.Name != name {
			l = append(l, s)
		}
	}
	c.snaps[nodename] = l
	return nil
}

func (c *fakeCluster) reclaimable(dataset, first, last string) (int64, error) {
	return c.held, nil
}

func (c *fakeCluster) available(pool string) (int64, error) {
	return c.avail, nil
}

func (c *fakeCluster) find(name string) snapshot {
	for _, s := range c.snaps[""] {
		if s.Name == name {
			return s
		}
	}
	panic("no snapshot " + name)
}

func (c *fakeCluster) received(s snapshot) snapshot {
	s.Name = c.t.Dst + "@" + snapShortName(s.Name)
	return s
}

func (c *fakeCluster) sendFull(ctx context.Context, nodename, snap string) error {
	if c.down[nodename] {
		return fmt.Errorf("%s is down", nodename)
	}
	s := c.find(snap)
	c.snaps[nodename] = nil
	for _, x := range c.snaps[""] {
		// a replication stream carries the older snapshots too
		if x.CreateTxg == s.CreateTxg || (c.t.Recursive && x.CreateTxg < s.CreateTxg) {
			c.snaps[nodename] = append(c.snaps[nodename], c.received(x))
		}
	}
	c.sent[nodename] = append(c.sent[nodename], "full "+snapShortName(snap))
	return nil
}

func (c *fakeCluster) sendIncremental(ctx context.Context, nodename, base, snap string) error {
	if c.down[nodename] {
		return fmt.Errorf("%s is down", nodename)
	}
	b, s := c.find(base), c.find(snap)
	if _, ok := findGUID(c.snaps[nodename], b.GUID); !ok {
		return fmt.Errorf("%s does not hold %s", nodename, base)
	}
	for _, x := range c.snaps[""] {
		if x.CreateTxg > b.CreateTxg && (x.CreateTxg == s.CreateTxg || (c.t.Intermediary && x.CreateTxg < s.CreateTxg)) {
			c.snaps[nodename] = append(c.snaps[nodename], c.received(x))
		}
	}
	c.sent[nodename] = append(c.sent[nodename], "incr "+snapShortName(base)+" "+snapShortName(snap))
	return nil
}

// run is what lockedSync does, without the object around it.
func (c *fakeCluster) run(mode modeT, now time.Time, states peerStates, peers, targets []string) error {
	name := c.t.newSnapName(now)
	if err := c.takeSnapshot(name); err != nil {
		return err
	}
	local := c.t.ownSnapshots(c.snaps[""])
	snap := local[len(local)-1]
	c.t.seedLegacyStates(local, peers, states)
	var errs error
	for _, nodename := range targets {
		st := states[nodename]
		err := c.t.syncPeer(context.Background(), mode, nodename, local, snap, &st, now)
		states[nodename] = st
		errs = errors.Join(errs, err)
	}
	return errors.Join(errs, c.t.pruneLocal(local, peers, states))
}

func (c *fakeCluster) names(nodename string) []string {
	var l []string
	for _, s := range c.snaps[nodename] {
		l = append(l, snapShortName(s.Name))
	}
	return l
}

func newTestT(t *testing.T) *T {
	d := &T{Src: "pool/fs", Dst: "pool/fs", Intermediary: true}
	require.NoError(t, d.SetRID("sync#1"))
	p, err := naming.ParsePath("test/svc/zfs")
	require.NoError(t, err)
	d.Path = p
	return d
}

var t0 = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func snapAt(h int) string {
	return "sync.1." + t0.Add(time.Duration(h)*time.Hour).Format("20060102T150405Z")
}

func TestSyncPeers(t *testing.T) {
	peers := []string{"n2", "n3"}

	t.Run("a short outage heals incrementally", func(t *testing.T) {
		d := newTestT(t)
		c := newFakeCluster(d)
		states := peerStates{}

		require.NoError(t, c.run(modeIncr, t0, states, peers, peers))
		require.Equal(t, []string{"full " + snapAt(0)}, c.sent["n2"], "never synced: full without asking")
		require.Equal(t, []string{snapAt(0)}, c.names(""))

		c.down["n3"] = true
		err := c.run(modeIncr, t0.Add(time.Hour), states, peers, peers)
		require.ErrorContains(t, err, "n3 is down")
		require.Equal(t, []string{snapAt(0), snapAt(1)}, c.names(""), "the base of n3 is kept")
		require.Equal(t, []string{snapAt(1)}, c.names("n2"), "n2 went on")
		require.Equal(t, t0.Add(time.Hour), states["n3"].FailedAt)

		c.down["n3"] = false
		require.NoError(t, c.run(modeIncr, t0.Add(2*time.Hour), states, peers, peers))
		require.Equal(t, "incr "+snapAt(0)+" "+snapAt(2), c.sent["n3"][1], "n3 sent the delta from its base")
		require.Equal(t, []string{snapAt(2)}, c.names(""))
		require.Equal(t, []string{snapAt(2)}, c.names("n2"))
		require.Equal(t, []string{snapAt(2)}, c.names("n3"))
		require.True(t, states["n3"].FailedAt.IsZero())
	})

	t.Run("a long outage strands the peer until a full", func(t *testing.T) {
		d := newTestT(t)
		c := newFakeCluster(d)
		states := peerStates{}
		require.NoError(t, c.run(modeIncr, t0, states, peers, peers))

		c.down["n3"] = true
		require.Error(t, c.run(modeIncr, t0.Add(time.Hour), states, peers, peers))
		err := c.run(modeIncr, t0.Add(26*time.Hour), states, peers, peers)
		require.ErrorIs(t, err, errStranded)
		require.ErrorContains(t, err, "max_lag_age")
		require.ErrorContains(t, err, "om test/svc/zfs instance full --rid sync#1 --target n3")
		require.Equal(t, []string{snapAt(26)}, c.names(""), "the base of n3 is released")

		c.down["n3"] = false
		err = c.run(modeIncr, t0.Add(27*time.Hour), states, peers, peers)
		require.ErrorIs(t, err, errStranded, "back, but not synced without asking")
		require.Equal(t, []string{snapAt(0)}, c.names("n3"))

		d.Recursive = true
		require.NoError(t, c.run(modeFull, t0.Add(28*time.Hour), states, peers, []string{"n3"}))
		require.Equal(t, []string{snapAt(28)}, c.names("n3"), "the older snapshots a recursive full brought are gone")
		require.True(t, states["n3"].StrandedAt.IsZero())
		require.Equal(t, []string{snapAt(27), snapAt(28)}, c.names(""), "n2 keeps its base, not synced by the full")

		require.NoError(t, c.run(modeIncr, t0.Add(29*time.Hour), states, peers, peers))
		require.Equal(t, []string{snapAt(29)}, c.names(""))
	})

	t.Run("a peer holding too much space is stranded", func(t *testing.T) {
		d := newTestT(t)
		c := newFakeCluster(d)
		states := peerStates{}
		require.NoError(t, c.run(modeIncr, t0, states, peers, peers))
		c.down["n3"] = true
		require.Error(t, c.run(modeIncr, t0.Add(time.Hour), states, peers, peers))

		// 20% of what the pool would have free without the hold
		c.avail, c.held = 790, 200
		err := c.run(modeIncr, t0.Add(2*time.Hour), states, peers, peers)
		require.ErrorIs(t, err, errStranded)
		require.ErrorContains(t, err, "max_lag_size")

		d2 := newTestT(t)
		d2.MaxLagSize = "1k"
		c2 := newFakeCluster(d2)
		states2 := peerStates{}
		require.NoError(t, c2.run(modeIncr, t0, states2, peers, peers))
		c2.down["n3"] = true
		require.Error(t, c2.run(modeIncr, t0.Add(time.Hour), states2, peers, peers))
		c2.held = 1000
		err = c2.run(modeIncr, t0.Add(2*time.Hour), states2, peers, peers)
		require.NotErrorIs(t, err, errStranded, "under 1k")
		c2.held = 1025
		err = c2.run(modeIncr, t0.Add(3*time.Hour), states2, peers, peers)
		require.ErrorIs(t, err, errStranded, "over 1k")
	})

	t.Run("a peer with no snapshot in common is not overwritten", func(t *testing.T) {
		d := newTestT(t)
		c := newFakeCluster(d)
		states := peerStates{}
		c.snaps["n2"] = []snapshot{c.newSnapshot("pool/fs@sync.1.other")}
		err := c.run(modeIncr, t0, states, peers, []string{"n2"})
		require.ErrorIs(t, err, errStranded)
		require.ErrorContains(t, err, "no snapshot in common")
		require.Empty(t, c.sent["n2"])
	})

	t.Run("a peer holding the snapshot already is not sent it again", func(t *testing.T) {
		d := newTestT(t)
		c := newFakeCluster(d)
		states := peerStates{}
		require.NoError(t, c.run(modeIncr, t0, states, peers, peers))
		name := d.newSnapName(t0.Add(time.Hour))
		require.NoError(t, c.takeSnapshot(name))
		c.snaps["n2"] = append(c.snaps["n2"], c.received(c.find(name)))
		local := d.ownSnapshots(c.snaps[""])
		st := states["n2"]
		require.NoError(t, d.syncPeer(context.Background(), modeIncr, "n2", local, local[len(local)-1], &st, t0.Add(time.Hour)))
		require.Len(t, c.sent["n2"], 1)
		require.Equal(t, local[len(local)-1].GUID, st.BaseGUID)
	})

	t.Run("the snapshots of an older agent are the base of its peers", func(t *testing.T) {
		d := newTestT(t)
		c := newFakeCluster(d)
		states := peerStates{}
		sent := c.newSnapshot("pool/fs@sync.1.sent")
		c.snaps[""] = []snapshot{sent}
		c.snaps["n2"] = []snapshot{c.received(sent)}
		c.snaps["n3"] = []snapshot{c.received(sent)}

		c.down["n3"] = true
		require.Error(t, c.run(modeIncr, t0, states, peers, peers))
		require.Equal(t, "incr sync.1.sent "+snapAt(0), c.sent["n2"][0])
		require.Equal(t, []string{"sync.1.sent", snapAt(0)}, c.names(""), "kept for n3, out of reach")

		c.down["n3"] = false
		require.NoError(t, c.run(modeIncr, t0.Add(time.Hour), states, peers, peers))
		require.Equal(t, "incr sync.1.sent "+snapAt(1), c.sent["n3"][0])
		require.Equal(t, []string{snapAt(1)}, c.names(""))
		require.Equal(t, []string{snapAt(1)}, c.names("n3"))
	})
}

func TestParseMaxLagSize(t *testing.T) {
	for s, ok := range map[string]bool{"20%": true, "0%": true, "100%": true, "101%": false, "x%": false, "10g": true, "1k": true, "abc": false} {
		err := validateMaxLagSize(s)
		require.Equal(t, ok, err == nil, s)
	}
}

func TestParseSnapshots(t *testing.T) {
	l, err := parseSnapshots([]byte("pool/fs@sync.1.a\t123\t10\npool/fs@sync.1.b\t456\t12\n"))
	require.NoError(t, err)
	require.Equal(t, []snapshot{{"pool/fs@sync.1.a", "123", 10}, {"pool/fs@sync.1.b", "456", 12}}, l)
	_, err = parseSnapshots([]byte("bad line\n"))
	require.Error(t, err)
}

func TestParseReclaim(t *testing.T) {
	out := "destroy\tpool/fs@sync.1.a\ndestroy\tpool/fs@sync.1.b\nreclaim\t123456\n"
	n, err := parseReclaim([]byte(out))
	require.NoError(t, err)
	require.Equal(t, int64(123456), n)
	_, err = parseReclaim([]byte(strings.Repeat("x\n", 2)))
	require.Error(t, err)
}
