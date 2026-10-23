package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"golang.org/x/crypto/ssh"
)

// Sharer is a store keeping its runs in a directory of the node: one every
// node reaches, as an nfs mount, when shared, or one per node otherwise, the
// directory of a peer reached over an ssh connection to it.
type Sharer interface {
	IsShared() bool
	OverSSH(conn *ssh.Client) error
}

// nodesStore is a store keeping its runs in the directory of each node: it
// writes the runs of this node into its own, lists the runs of every node, and
// reads, completes and deletes a run in the directory of the node that wrote
// it, reached over ssh for a peer. A run written on one node is restored on
// another, without passing through a third.
type nodesStore struct {
	Driver
	local   string
	peers   []string
	newPeer func(node string) (Driver, error)
	warnf   func(string, ...any)
	opened  map[string]Driver
	failed  map[string]error
}

var (
	_ Driver   = (*nodesStore)(nil)
	_ Streamer = (*nodesStore)(nil)
	_ Pruner   = (*nodesStore)(nil)
)

// NewNodesStore returns the store local, the driver of the directory of the
// node named local, spread over the directories of the peers, each reached
// by the driver newPeer returns. warnf says the peers whose runs are not
// listed.
func NewNodesStore(driver Driver, local string, peers []string, newPeer func(string) (Driver, error), warnf func(string, ...any)) Driver {
	return &nodesStore{
		Driver:  driver,
		local:   local,
		peers:   peers,
		newPeer: newPeer,
		warnf:   warnf,
		opened:  make(map[string]Driver),
		failed:  make(map[string]error),
	}
}

// driverOf returns the driver of the directory of node, connected once.
func (t *nodesStore) driverOf(node string) (Driver, error) {
	if node == "" || node == t.local {
		return t.Driver, nil
	}
	if d, ok := t.opened[node]; ok {
		return d, nil
	}
	if err, ok := t.failed[node]; ok {
		return nil, err
	}
	d, err := t.newPeer(node)
	if err != nil {
		err = fmt.Errorf("store %s: the directory of %s is not reachable: %w", t.Ref(), node, err)
		t.failed[node] = err
		return nil, err
	}
	t.opened[node] = d
	return d, nil
}

// Runs returns the runs filter selects in the directories of every node,
// oldest first. A peer not reachable is said, and its runs are not listed.
func (t *nodesStore) Runs(ctx context.Context, filter RunFilter) ([]Run, error) {
	runs, err := t.Driver.Runs(ctx, filter)
	if err != nil {
		return runs, err
	}
	for _, node := range t.peers {
		d, err := t.driverOf(node)
		if err != nil {
			t.warnf("%s: its runs are not listed", err)
			continue
		}
		l, err := d.Runs(ctx, filter)
		if err != nil {
			t.warnf("store %s: the runs of %s: %s", t.Ref(), node, err)
		}
		runs = append(runs, l...)
	}
	slices.SortStableFunc(runs, func(a, b Run) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return runs, nil
}

func (t *nodesStore) Restore(ctx context.Context, run Run, member string, dst string) error {
	d, err := t.driverOf(run.Node)
	if err != nil {
		return err
	}
	return d.Restore(ctx, run, member, dst)
}

func (t *nodesStore) Complete(ctx context.Context, run Run) error {
	d, err := t.driverOf(run.Node)
	if err != nil {
		return err
	}
	return d.Complete(ctx, run)
}

func (t *nodesStore) BackupStream(ctx context.Context, run Run, member string, r io.Reader) error {
	streamer, ok := t.Driver.(Streamer)
	if !ok {
		return fmt.Errorf("store %s takes no streams", t.Ref())
	}
	return streamer.BackupStream(ctx, run, member, r)
}

func (t *nodesStore) Cat(ctx context.Context, run Run, member string) (io.ReadCloser, error) {
	d, err := t.driverOf(run.Node)
	if err != nil {
		return nil, err
	}
	streamer, ok := d.(Streamer)
	if !ok {
		return nil, fmt.Errorf("store %s takes no streams", t.Ref())
	}
	return streamer.Cat(ctx, run, member)
}

// Delete deletes each run in the directory of the node that wrote it.
func (t *nodesStore) Delete(ctx context.Context, runs []Run) error {
	byNode := make(map[string][]Run)
	for _, run := range runs {
		byNode[run.Node] = append(byNode[run.Node], run)
	}
	var errs error
	for node, l := range byNode {
		d, err := t.driverOf(node)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		pruner, ok := d.(Pruner)
		if !ok {
			return fmt.Errorf("store %s removes no run", t.Ref())
		}
		errs = errors.Join(errs, pruner.Delete(ctx, l))
	}
	return errs
}

// Check verifies the runs filter selects in the directories of every node.
func (t *nodesStore) Check(ctx context.Context, filter RunFilter) error {
	errs := t.Driver.Check(ctx, filter)
	for _, node := range t.peers {
		d, err := t.driverOf(node)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		errs = errors.Join(errs, d.Check(ctx, filter))
	}
	return errs
}

// Close closes the connections to the peers, and the driver of this node.
func (t *nodesStore) Close() error {
	errs := t.Driver.Close()
	for _, d := range t.opened {
		errs = errors.Join(errs, d.Close())
	}
	return errs
}

// IsLocal says the store is a directory of the node, as its driver is.
func (t *nodesStore) IsLocal() bool {
	return true
}
