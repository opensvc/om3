package object

import (
	"context"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pg"
)

type (
	nscfg struct {
		pg *pg.Config
		core
	}

	Nscfg interface {
		Core
		PG() *pg.Config
	}
)

func NewNscfg(path naming.Path, opts ...funcopt.O) (*nscfg, error) {
	s := &nscfg{}
	s.path = path
	s.path.Kind = naming.KindNscfg
	err := s.init(s, path, opts...)
	return s, err
}

func (t *nscfg) KeywordLookup(k key.T, sectionType string) *keywords.Keyword {
	return keywordLookup(keywordStore, k, t.path.Kind, sectionType)
}

func (t *nscfg) PGUpdate(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.PGUpdate)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedPGUpdate(ctx)
}

func (t *nscfg) PGConfig() *pg.Config {
	// For nscfg, we want to control the namespace cgroup, not the object cgroup
	data := t.pgAnonConfig("")

	// Build the namespace cgroup path only
	// The ID must match the expected format for both cgroups v1 and v2
	// For v2: relative path like "opensvc-ns.test.slice" (no leading slash, dots in name)
	// For v1: full path like "/opensvc.slice/opensvc-ns.test.slice" (leading slash, slashes for hierarchy)
	// We'll use the v1 format with leading slash, which should work for both
	if t.path.Namespace == naming.NsRoot {
		data.ID = "/opensvc.slice"
	} else {
		ns := pgNameNamespace(t.path.Namespace)
		data.ID = "/opensvc.slice/opensvc-" + ns + ".slice"
	}
	return data.WithLogger(t.log)
}

func (t *nscfg) lockedPGUpdate(ctx context.Context) error {
	// For nscfg, we control the namespace cgroup, not the object cgroup
	return t.PGConfig().Apply()
}
