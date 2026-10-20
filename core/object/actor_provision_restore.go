package object

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/statusbus"
	"github.com/opensvc/om3/v3/util/key"
)

// provisionRestore is the snapshot a provision restores the heads it
// provisions from, as the restore_from keyword names it.
type provisionRestore struct {
	store backup.Store
	run   backup.Run
}

// provisionRestoreSource returns the snapshot restore_from names, nil when it
// names none.
func (t *actor) provisionRestoreSource(ctx context.Context) (*provisionRestore, error) {
	s := t.config.GetString(key.Parse("restore_from"))
	if s == "" {
		return nil, nil
	}
	fields := strings.Fields(s)
	if len(fields) != 3 {
		return nil, fmt.Errorf("restore_from %q: want <store> <object path> <snapshot id>", s)
	}
	n, err := t.Node()
	if err != nil {
		return nil, err
	}
	store, err := n.Store(fields[0])
	if err != nil {
		return nil, fmt.Errorf("restore_from: %w", err)
	}
	run, err := backup.FindRun(ctx, store, fields[1], fields[2])
	if err != nil {
		return nil, fmt.Errorf("restore_from: %w", err)
	}
	return &provisionRestore{store: store, run: run}, nil
}

// restoreProvisionedHead restores into the head of r the member of the
// snapshot named after it, r being one the provision moved from
// unprovisioned to provisioned: its head holds nothing yet. A head the
// provision did not start, as a shared one off the placement leader, is left
// to the node starting it.
func (t *actor) restoreProvisionedHead(ctx context.Context, src *provisionRestore, r resource.Driver) error {
	rid := r.RID()
	if !src.run.Has(rid) {
		return nil
	}
	h, ok := r.(interface{ Head() string })
	if !ok {
		return nil
	}
	dir := h.Head()
	switch st := statusbus.FromContext(ctx).Get(rid); {
	case dir == "":
		return fmt.Errorf("restore %s: no head directory", rid)
	case st.Is(status.Up, status.StandbyUp):
	case st.Is(status.NotApplicable):
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.log.Attr("rid", rid).Infof("%s: skip the restore, its head %s is not there", rid, dir)
			return nil
		}
	default:
		t.log.Attr("rid", rid).Infof("%s: skip the restore, not started here", rid)
		return nil
	}
	t.log.Attr("rid", rid).Infof("%s: restore from snapshot %s of %s in store %s into %s", rid, src.run.ShortID(), src.run.Path, src.store.Ref(), dir)
	if err := src.store.Restore(ctx, src.run, rid, dir); err != nil {
		return fmt.Errorf("restore %s: %w", rid, err)
	}
	return nil
}
