package zfs

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMoveRunner answers the zfs commands of a move from the snapshots each
// node holds, and records the commands it ran.
type fakeMoveRunner struct {
	localSnaps  string
	remoteSnaps string
	remoteHas   bool
	ran         []string
}

func (t *fakeMoveRunner) local(_ context.Context, args ...string) ([]byte, error) {
	t.ran = append(t.ran, "local zfs "+strings.Join(args, " "))
	if args[0] == "list" {
		return []byte(t.localSnaps), nil
	}
	return nil, nil
}

func (t *fakeMoveRunner) remote(_ context.Context, cmdline string) ([]byte, error) {
	t.ran = append(t.ran, "remote "+cmdline)
	switch {
	case strings.HasPrefix(cmdline, "/usr/sbin/zfs list -H -o name "):
		if !t.remoteHas {
			return nil, fmt.Errorf("%s: exit 1: dataset does not exist", cmdline)
		}
		return []byte("tank/vm\n"), nil
	case strings.HasPrefix(cmdline, "/usr/sbin/zfs list"):
		return []byte(t.remoteSnaps), nil
	}
	return nil, nil
}

func (t *fakeMoveRunner) pipe(_ context.Context, send []string, receive string) error {
	t.ran = append(t.ran, "pipe "+strings.Join(send, " ")+" | "+receive)
	return nil
}

func (t *fakeMoveRunner) piped() string {
	for _, s := range t.ran {
		if strings.HasPrefix(s, "pipe ") {
			return s
		}
	}
	return ""
}

func TestMoverSend(t *testing.T) {
	t.Run("a destination without the dataset receives it in full", func(t *testing.T) {
		r := &fakeMoveRunner{}
		m := &Mover{Name: "tank/vm", Node: "n2", run: r}
		snap, err := m.Send(context.Background())
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(snap, "tank/vm@"+MoveSnapshotPrefix), snap)
		assert.Equal(t, "pipe /usr/sbin/zfs send -p "+snap+" | /usr/sbin/zfs receive -u tank/vm", r.piped())
	})

	t.Run("the parents of a nested dataset are created", func(t *testing.T) {
		r := &fakeMoveRunner{}
		m := &Mover{Name: "tank/vms/vm1", Node: "n2", run: r}
		_, err := m.Send(context.Background())
		require.NoError(t, err)
		assert.Contains(t, r.ran, "remote /usr/sbin/zfs create -p tank/vms")
	})

	t.Run("the newest snapshot in common is the base", func(t *testing.T) {
		r := &fakeMoveRunner{
			localSnaps:  "tank/vm@a\t1\t10\ntank/vm@b\t2\t20\ntank/vm@c\t3\t30\n",
			remoteSnaps: "tank/vm@a\t1\t5\ntank/vm@b\t2\t6\n",
			remoteHas:   true,
		}
		m := &Mover{Name: "tank/vm", Node: "n2", run: r}
		snap, err := m.Send(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "pipe /usr/sbin/zfs send -p -i tank/vm@b "+snap+" | /usr/sbin/zfs receive -u -F tank/vm", r.piped())
	})

	t.Run("a destination holding the dataset with nothing in common is refused", func(t *testing.T) {
		r := &fakeMoveRunner{
			localSnaps:  "tank/vm@a\t1\t10\n",
			remoteSnaps: "tank/vm@x\t9\t5\n",
			remoteHas:   true,
		}
		m := &Mover{Name: "tank/vm", Node: "n2", run: r}
		_, err := m.Send(context.Background())
		require.ErrorContains(t, err, "no snapshot in common")
		assert.Empty(t, r.piped(), "nothing sent")
		for _, s := range r.ran {
			assert.NotContains(t, s, "snapshot tank/vm@", "no snapshot taken")
		}
	})
}

// A prune destroys the move snapshots but the one kept, here and on the
// destination, and leaves the snapshots of other names alone.
func TestMoverPrune(t *testing.T) {
	keep := "tank/vm@" + MoveSnapshotPrefix + "2"
	r := &fakeMoveRunner{
		localSnaps:  "tank/vm@" + MoveSnapshotPrefix + "1\t1\t10\ntank/vm@sync\t2\t20\n" + keep + "\t3\t30\n",
		remoteSnaps: "tank/vm@" + MoveSnapshotPrefix + "1\t1\t5\n" + keep + "\t3\t6\n",
		remoteHas:   true,
	}
	m := &Mover{Name: "tank/vm", Node: "n2", run: r}
	require.NoError(t, m.Prune(context.Background(), keep))
	assert.Contains(t, r.ran, "local zfs destroy tank/vm@"+MoveSnapshotPrefix+"1")
	assert.Contains(t, r.ran, "remote /usr/sbin/zfs destroy tank/vm@"+MoveSnapshotPrefix+"1")
	for _, s := range r.ran {
		assert.NotContains(t, s, "destroy "+keep)
		assert.NotContains(t, s, "destroy tank/vm@sync")
	}
}
