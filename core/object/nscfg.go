package object

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pg"
)

type (
	nscfg struct {
		actor
	}

	Nscfg interface {
		Actor
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

func (t *nscfg) Schedules() (l schedule.Table) {
	return
}

func (t *nscfg) Boot(ctx context.Context) error {
	ctx = actioncontext.WithProps(ctx, actioncontext.Boot)
	if err := t.validateAction(); err != nil {
		return err
	}
	t.setenv("boot", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	return t.lockedBoot(ctx)
}

func (t *nscfg) lockedBoot(ctx context.Context) error {
	// For nscfg, boot action calls PGUpdate instead of resource.Boot
	return t.PGUpdate(ctx)
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

func (t *nscfg) pgConfigNamespace() *pg.Config {
	// For nscfg, we want to control the namespace cgroup, not the object cgroup
	data := pg.Config{}
	data.CPUShares, _ = t.config.EvalNoConv(key.New("", "pg_cpu_shares"))
	data.CPUs, _ = t.config.EvalNoConv(key.New("", "pg_cpus"))
	data.Mems, _ = t.config.EvalNoConv(key.New("", "pg_mems"))
	data.CPUQuota, _ = t.config.EvalNoConv(key.New("", "pg_cpu_quota"))
	data.MemLimit, _ = t.config.EvalNoConv(key.New("", "pg_mem_limit"))
	data.VMemLimit, _ = t.config.EvalNoConv(key.New("", "pg_vmem_limit"))
	data.MemOOMControl, _ = t.config.EvalNoConv(key.New("", "pg_mem_oom_control"))
	data.MemSwappiness, _ = t.config.EvalNoConv(key.New("", "pg_mem_swappiness"))
	data.BlockIOWeight, _ = t.config.EvalNoConv(key.New("", "pg_blkio_weight"))

	// Build the namespace cgroup path only
	// The ID must match the expected format for both cgroups v1 and v2
	// For v2: relative path like "opensvc-ns.test.slice" (no leading slash, dots in name)
	// For v1: full path like "/opensvc.slice/opensvc-ns.test.slice" (leading slash, slashes for hierarchy)
	// We'll use the v1 format with leading slash, which should work for both
	if t.path.Namespace == naming.NsRoot {
		data.ID = "/opensvc.slice/opensvc.slice"
	} else {
		ns := pgNameNamespace(t.path.Namespace)
		data.ID = "/opensvc.slice/opensvc-" + ns + ".slice"
	}
	return &data
}

func (t *nscfg) lockedPGUpdate(ctx context.Context) error {
	// For nscfg, we control the namespace cgroup, not the object cgroup
	pgConfig := t.pgConfigNamespace()
	if pgConfig == nil {
		return nil
	}
	mgr := pg.FromContext(ctx)
	if mgr == nil {
		return nil
	}
	mgr.Register(pgConfig)
	for _, run := range mgr.Apply(pgConfig.ID) {
		if !run.Changed {
			continue
		}
		if configStr := run.Config.String(); strings.Contains(configStr, "=") {
			t.log.Infof("applied %s", configStr)
		} else {
			t.log.Tracef("create %s", configStr)
		}
		if run.Err != nil {
			return run.Err
		}
	}
	return nil
}
