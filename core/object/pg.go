package object

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pg"
	"github.com/opensvc/om3/util/stringslice"
)

func pgNameObject(p naming.Path) string {
	return fmt.Sprintf("%s.%s", p.Kind, p.Name)
}

func pgNameSubset(s string) string {
	return fmt.Sprintf("subset.%s", strings.ReplaceAll(s, ":", "."))
}

func pgNameResource(s string) string {
	return strings.ReplaceAll(s, "#", ".")
}

// /opensvc/ns.ns1/vol.v1/subset.g1/disk.1 	# ns ss
// /opensvc/ns.ns1/vol.v1/subset.g1/fs.1 	# ns ss
// /opensvc/ns.ns1/vol.v1/subset.g1/fs.g1	# ns ss
// /opensvc/ns.ns1/vol.v1/fs.g1	 		# ns !ss
// /opensvc/ns.ns1/vol.v1/disk.1		# ns !ss
// /opensvc/vol.v1/subset.g1/disk.1		# !ns ss
// /opensvc/vol.v1/subset.g1/app.1		# !ns ss
// /opensvc/vol.v1/disk.1			# !ns !ss
func (t *core) pgConfig(section string) *pg.Config {
	data := pg.Config{}
	data.CPUShares, _ = t.config.EvalNoConv(key.New(section, "pg_cpu_shares"))
	data.CPUs, _ = t.config.EvalNoConv(key.New(section, "pg_cpus"))
	data.Mems, _ = t.config.EvalNoConv(key.New(section, "pg_mems"))
	data.CPUQuota, _ = t.config.EvalNoConv(key.New(section, "pg_cpu_quota"))
	data.MemLimit, _ = t.config.EvalNoConv(key.New(section, "pg_mem_limit"))
	data.VMemLimit, _ = t.config.EvalNoConv(key.New(section, "pg_vmem_limit"))
	data.MemOOMControl, _ = t.config.EvalNoConv(key.New(section, "pg_mem_oom_control"))
	data.MemSwappiness, _ = t.config.EvalNoConv(key.New(section, "pg_mem_swappiness"))
	data.BlockIOWeight, _ = t.config.EvalNoConv(key.New(section, "pg_blkio_weight"))
	subsetName := func(s string) string {
		if s == "" {
			return ""
		}
		l := strings.SplitN(s, ":", 2)
		n := len(l)
		switch n {
		case 2:
			return pgNameSubset(l[n-1])
		case 1:
			return pgNameSubset(s)
		default:
			return ""
		}
	}
	svcPGName := func() []string {
		s := pgNameObject(t.path)
		if t.path.Namespace == "root" {
			return []string{"opensvc", s}
		}
		return []string{"opensvc", t.path.Namespace, s}
	}
	subsetPGName := func(s string) []string {
		name := subsetName(s)
		if name == "" {
			return svcPGName()
		}
		return append(svcPGName(), name)
	}
	resPGName := func(s string) []string {
		ss, _ := t.config.EvalNoConv(key.New(section, "subset"))
		return append(subsetPGName(ss), pgNameResource(s))
	}
	pgName := func(s string) string {
		var l []string
		switch {
		case section == "":
			l = svcPGName()
		case strings.HasPrefix(s, "subset#"):
			l = subsetPGName(s[7:])
		default:
			l = resPGName(s)
		}
		l = stringslice.Map(l, func(s string) string {
			return s + ".slice"
		})
		return "/" + strings.Join(l, "/")
	}
	data.ID = pgName(section)
	return &data
}

func (t *core) CleanPG(ctx context.Context) {
	mgr := pg.FromContext(ctx)
	if mgr == nil {
		return
	}
	for _, run := range mgr.Clean() {
		if run.Err != nil {
			t.log.Errorf("clean pg %s: %s", run.Config.ID, run.Err)
		} else if run.Changed {
			t.log.Infof("clean pg %s", run.Config.ID)
		}
	}
}
