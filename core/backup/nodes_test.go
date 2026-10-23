package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore is the directory of a store on one node, in memory.
type fakeStore struct {
	T
	node     string
	runs     []Run
	restored []string
	deleted  []string
}

func (t *fakeStore) Backup(context.Context, Run, string, string, string, []string) error { return nil }
func (t *fakeStore) Complete(context.Context, Run) error                                 { return nil }
func (t *fakeStore) Check(context.Context, RunFilter) error                              { return nil }

func (t *fakeStore) Runs(_ context.Context, filter RunFilter) ([]Run, error) {
	l := make([]Run, 0)
	for _, run := range t.runs {
		if filter.Match(run) {
			l = append(l, run)
		}
	}
	return l, nil
}

func (t *fakeStore) Restore(_ context.Context, run Run, member, _ string) error {
	t.restored = append(t.restored, run.ID+":"+member)
	return nil
}

func (t *fakeStore) Delete(_ context.Context, runs []Run) error {
	for _, run := range runs {
		t.deleted = append(t.deleted, run.ID)
	}
	return nil
}

func (t *fakeStore) BackupStream(context.Context, Run, string, io.Reader) error { return nil }

func (t *fakeStore) Cat(context.Context, Run, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(t.node)), nil
}

func fakeRun(id, node string, at time.Time) Run {
	return Run{ID: id, Node: node, ObjectID: "oid", ClusterID: "c1", CreatedAt: at, CompletedAt: at}
}

// A store with a directory per node lists the runs of every node, and reads
// or deletes a run in the directory of the node that wrote it. A peer not
// reachable is said, its runs not listed, and a run of it not read.
func TestNodesStore(t *testing.T) {
	t0 := time.Date(2026, 10, 23, 2, 0, 0, 0, time.UTC)
	n1 := &fakeStore{node: "n1", runs: []Run{fakeRun("a", "n1", t0)}}
	n2 := &fakeStore{node: "n2", runs: []Run{fakeRun("b", "n2", t0.Add(time.Hour))}}
	warnings := make([]string, 0)
	s := NewNodesStore(n1, "n1", []string{"n2", "n3"}, func(node string) (Driver, error) {
		if node == "n2" {
			return n2, nil
		}
		return nil, errors.New("connection refused")
	}, func(format string, args ...any) { warnings = append(warnings, fmt.Sprintf(format, args...)) })
	ctx := context.Background()

	runs, err := s.Runs(ctx, RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, "a", runs[0].ID)
	assert.Equal(t, "b", runs[1].ID)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "the directory of n3 is not reachable")

	require.NoError(t, s.Restore(ctx, runs[1], "fs#1", "/tmp/x"))
	assert.Equal(t, []string{"b:fs#1"}, n2.restored)
	assert.Empty(t, n1.restored)
	rc, err := s.(Streamer).Cat(ctx, runs[1], "config")
	require.NoError(t, err)
	b, _ := io.ReadAll(rc)
	assert.Equal(t, "n2", string(b), "read on the node that wrote it")

	require.NoError(t, s.(Pruner).Delete(ctx, runs))
	assert.Equal(t, []string{"a"}, n1.deleted)
	assert.Equal(t, []string{"b"}, n2.deleted)

	err = s.Restore(ctx, fakeRun("c", "n3", t0), "fs#1", "/tmp/x")
	assert.ErrorContains(t, err, "the directory of n3 is not reachable")
}
