package omcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	CmdStoreSnapshotRm struct {
		OptsGlobal
		Store    string
		Snapshot string
	}
)

// Run removes the run the snapshot id names from the store, protected or
// not: the retention never removes a protected one.
func (t *CmdStoreSnapshotRm) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	s, err := n.Store(t.Store, naming.Path{})
	if err != nil {
		return err
	}
	defer s.Close()
	pruner, ok := s.(backup.Pruner)
	if !ok {
		return fmt.Errorf("store %s removes no run: its runs go as its server decides", s.Ref())
	}
	ctx := context.Background()
	run, err := backup.FindRun(ctx, s, backup.RunFilter{ID: t.Snapshot, Incomplete: true})
	if err != nil {
		return err
	}
	if err := pruner.Delete(ctx, []backup.Run{run}); err != nil {
		return err
	}
	what := "run"
	if run.Protected {
		what = "protected run"
	}
	fmt.Printf("store %s: removed the %s %s of %s, from %s\n", s.Ref(), what, run.ID, run.Path, run.ClusterName)
	return nil
}
