package daemonapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/key"
)

// storeSource is the snapshot an object is created from: its configuration
// is the one of the object the snapshot is of, without the snapshot that
// one was created from, if any.
type storeSource struct {
	store  backup.Driver
	run    backup.Run
	config []byte
}

var keyRestoreFrom = key.Parse("restore_from")

// openStoreSource returns the snapshot store and snapshot name, read as the
// object p, which may be of an object of another namespace: the requester is
// an administrator of the namespace of that one, which may be read out of
// the snapshot.
func openStoreSource(ctx echo.Context, p naming.Path, store *api.InQueryStore, snapshot *api.InQueryCreateSnapshot) (*storeSource, int, error) {
	switch {
	case store == nil || *store == "":
		return nil, http.StatusBadRequest, fmt.Errorf("a snapshot needs the store holding it")
	case snapshot == nil || *snapshot == "":
		return nil, http.StatusBadRequest, fmt.Errorf("a store needs the snapshot: an id, or the path of an object whose latest snapshot to take")
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	s, err := n.Store(*store, p)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	src := &storeSource{store: s}
	reqCtx := ctx.Request().Context()
	clusterID, _ := n.Cluster()
	src.run, err = backup.FindSnapshot(reqCtx, s, *snapshot, clusterID)
	switch {
	case errors.Is(err, backup.ErrNotFound):
		src.close()
		return nil, http.StatusNotFound, err
	case errors.Is(err, backup.ErrAmbiguous):
		src.close()
		return nil, http.StatusBadRequest, err
	case err != nil:
		src.close()
		return nil, http.StatusInternalServerError, err
	}
	if !mayUseRun(grantsFromContext(ctx), *store, src.run, clusterID, rbac.RoleAdmin) {
		src.close()
		return nil, http.StatusForbidden, fmt.Errorf("snapshot %s is of %s on the cluster %s: creating an object from it needs the admin grant of its namespace on this cluster, or root", src.run.ShortID(), src.run.Path, src.run.ClusterName)
	}
	b, err := backup.ReadMember(reqCtx, s, src.run, backup.MemberConfig)
	if err != nil {
		src.close()
		return nil, http.StatusInternalServerError, fmt.Errorf("read the configuration of snapshot %s: %w", src.run.ShortID(), err)
	}
	if src.config, err = withoutRestoreFrom(p, b); err != nil {
		src.close()
		return nil, http.StatusBadRequest, fmt.Errorf("the configuration of snapshot %s: %w", src.run.ShortID(), err)
	}
	return src, 0, nil
}

// withoutRestoreFrom returns the configuration b without the snapshot the
// object it was saved from was created from: the one created from it is
// created from another.
func withoutRestoreFrom(p naming.Path, b []byte) ([]byte, error) {
	oc, err := object.NewConfigurer(p, object.WithConfigData(b), object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	cfg := oc.Config()
	if err := cfg.PrepareUnset(keyRestoreFrom); err != nil {
		return nil, err
	}
	return cfg.Dump()
}

// apply returns the configuration body naming the snapshot the provision
// restores the heads from, and the base landing it with the keys the
// snapshot holds that the namespace of p lacks.
//
// The keys are written as the configuration lands, once it passed every
// check, and taken back if it does not land: a create refused, by a claim, a
// hold or a configuration changed meanwhile, leaves no key of the snapshot in
// the namespace.
func (t *storeSource) apply(ctx echo.Context, p naming.Path, body []byte, base configBase) (object.Configurer, []byte, configBase, error) {
	log := LogHandler(ctx, "PostObjectConfigFile")
	if t.run.Has(backup.MemberRefs) {
		doc, err := t.readRefs(ctx.Request().Context())
		if err != nil {
			return nil, nil, base, err
		}
		base = base.landing(func() (func(bool), error) {
			notes, undo, err := object.ApplyRefsUndoable(p.Namespace, doc, object.RefsMissing)
			for _, note := range notes {
				log.Infof("%s: refs: %s", p, note)
			}
			done := func(landed bool) {
				if landed {
					return
				}
				if err := undo(); err != nil {
					log.Warnf("%s: the configuration was not written, and the keys copied for it from snapshot %s were not all taken back: %s", p, t.run.ShortID(), err)
				} else if len(notes) > 0 {
					log.Infof("%s: the configuration was not written, the keys copied for it from snapshot %s are taken back", p, t.run.ShortID())
				}
			}
			return done, err
		})
	}
	value := fmt.Sprintf("%s %s %s", t.store.Ref(), t.run.Path, t.run.ID)
	log.Infof("%s: the configuration of %s from snapshot %s in store %s, its heads restored at provision", p, t.run.Path, t.run.ShortID(), t.store.Ref())
	oc, body, err := object.CreatedConfig(p, body, true, *keyop.New(keyRestoreFrom, keyop.Set, value, 0))
	return oc, body, base, err
}

func (t *storeSource) readRefs(ctx context.Context) (object.RefsDocument, error) {
	b, err := backup.ReadMember(ctx, t.store, t.run, backup.MemberRefs)
	if err != nil {
		return nil, fmt.Errorf("read the referenced keys of snapshot %s: %w", t.run.ShortID(), err)
	}
	var doc object.RefsDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", backup.MemberRefs, err)
	}
	return doc, nil
}

func (t *storeSource) close() {
	_ = t.store.Close()
}
