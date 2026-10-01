package hbdisk

import (
	"context"
	"errors"
	"os"
	"slices"
	"syscall"
	"time"

	"github.com/ncw/directio"
)

// BeatingForeignNodes returns the nodes, other than nodes, whose heartbeat
// writes to the disk at dev, sorted.
//
// A disk can hold the heartbeats of several clusters, each node in a slot of
// its own, under one signature. Wiping the signature for one cluster takes it
// from all the others: their heartbeats find it gone at their next signature
// check, and can not start again until the disk is signed anew.
//
// A node beats when it writes its data slot within window: the time it
// stamped its write with is read twice, window apart, and a beating node
// has changed it in between. The age of that time against the clock of this
// node says nothing, as the nodes of another cluster may keep another time.
func BeatingForeignNodes(ctx context.Context, dev string, maxSlots int, nodes []string, window time.Duration) ([]string, error) {
	f, err := directio.OpenFile(dev, os.O_RDONLY, 0)
	if errors.Is(err, syscall.EINVAL) {
		// A filesystem refusing direct i/o, which no device does.
		f, err = os.Open(dev)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := device{path: dev, file: f, metaSize: metaSize(maxSlots)}

	foreign := make(map[int]string)
	for slot := minimumSlot; slot < maxSlots; slot++ {
		b, err := d.readMetaSlot(slot)
		if err != nil {
			return nil, err
		}
		if name := nodeFromMetadata(b); name != "" && !slices.Contains(nodes, name) {
			foreign[slot] = name
		}
	}
	if len(foreign) == 0 {
		return nil, nil
	}

	first := d.dataSlotUpdates(foreign)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(window):
	}
	second := d.dataSlotUpdates(foreign)

	l := make([]string, 0)
	for slot, name := range foreign {
		if updated, ok := second[slot]; ok && !updated.Equal(first[slot]) {
			l = append(l, name)
		}
	}
	slices.Sort(l)
	return slices.Compact(l), nil
}

// dataSlotUpdates returns the time the nodes of the slots stamped their last
// write with. A slot never written, or not read, has none: it shows no beat.
func (t *device) dataSlotUpdates(slots map[int]string) map[int]time.Time {
	m := make(map[int]time.Time, len(slots))
	for slot := range slots {
		if c, err := t.readDataSlot(slot); err == nil && !c.Updated.IsZero() {
			m[slot] = c.Updated
		}
	}
	return m
}
