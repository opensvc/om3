package object

import (
	"context"
	"fmt"
	"strings"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/pg"
	"opensvc.com/opensvc/util/stringslice"
)

func pgNameObject(p path.T) string {
	return fmt.Sprintf("%s.%s", p.Kind, p.Name)
}

func pgNameSubset(s string) string {
	return fmt.Sprintf("subset.%s", strings.ReplaceAll(s, ":", "."))
}

func pgNameResource(s string) string {
	return strings.ReplaceAll(s, "#", ".")
}

//
// /opensvc/ns.ns1/vol.v1/subset.g1/disk.1 	# ns ss
// /opensvc/ns.ns1/vol.v1/subset.g1/fs.1 	# ns ss
// /opensvc/ns.ns1/vol.v1/subset.g1/fs.g1	# ns ss
// /opensvc/ns.ns1/vol.v1/fs.g1	 		# ns !ss
// /opensvc/ns.ns1/vol.v1/disk.1		# ns !ss
// /opensvc/vol.v1/subset.g1/disk.1		# !ns ss
// /opensvc/vol.v1/subset.g1/app.1		# !ns ss
// /opensvc/vol.v1/disk.1			# !ns !ss
//
func (t *core) pgConfig(section string) *pg.Config {
	data := pg.Config{}
	data.CpuShares, _ = t.config.EvalNoConv(key.New(section, "pg_cpu_shares"))
	data.Cpus, _ = t.config.EvalNoConv(key.New(section, "pg_cpus"))
	data.Mems, _ = t.config.EvalNoConv(key.New(section, "pg_mems"))
	data.CpuQuota, _ = t.config.EvalNoConv(key.New(section, "pg_cpu_quota"))
	data.MemLimit, _ = t.config.EvalNoConv(key.New(section, "pg_mem_limit"))
	data.VMemLimit, _ = t.config.EvalNoConv(key.New(section, "pg_vmem_limit"))
	data.MemOOMControl, _ = t.config.EvalNoConv(key.New(section, "pg_mem_oom_control"))
	data.MemSwappiness, _ = t.config.EvalNoConv(key.New(section, "pg_mem_swappiness"))
	data.BlkioWeight, _ = t.config.EvalNoConv(key.New(section, "pg_blkio_weight"))
	subsetName := func(s string) string {
		l := strings.SplitN(s, ":", 2)
		n := len(l)
		switch n {
		case 0:
			return ""
		default:
			return pgNameSubset(l[n-1])
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
		case strings.HasPrefix(section, "subset#"):
			l = subsetPGName(s)
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
	for _, run := range pg.FromContext(ctx).Clean() {
		if run.Err != nil {
			t.log.Error().Err(run.Err).Msgf("clean pg %s", run.Config.ID)
		} else if run.Changed {
			t.log.Info().Msgf("clean pg %s", run.Config.ID)
		}
	}
}
