package nmon

import (
	"runtime"
	"strings"

	"github.com/prometheus/procfs"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

func (t *Manager) getNodeConfig() node.Config {
	var (
		keyMaintenanceGracePeriod = key.New("node", "maintenance_grace_period")
		keyMaxParallel            = key.New("node", "max_parallel")
		keyMaxKeySize             = key.New("node", "max_key_size")
		keyReadyPeriod            = key.New("node", "ready_period")
		keyRejoinGracePeriod      = key.New("node", "rejoin_grace_period")
		keyEnv                    = key.New("node", "env")
		keySplitAction            = key.New("node", "split_action")
		keySSHKey                 = key.New("node", "sshkey")
		keyPRKey                  = key.New("node", "prkey")
		keyMinAvailMem            = key.New("node", "min_avail_mem")
		keyMinAvailSwap           = key.New("node", "min_avail_swap")
	)
	cfg := node.Config{}
	cfg.Labels = t.config.SectionMap("labels")
	if d := t.config.GetDuration(keyMaintenanceGracePeriod); d != nil {
		cfg.MaintenanceGracePeriod = *d
	}
	if d := t.config.GetDuration(keyReadyPeriod); d != nil {
		cfg.ReadyPeriod = *d
	}
	if d := t.config.GetDuration(keyRejoinGracePeriod); d != nil {
		cfg.RejoinGracePeriod = *d
	}
	if d := t.config.GetSize(keyMaxKeySize); d != nil {
		cfg.MaxKeySize = *d
	}
	memTotal, swapTotal := memTotals()
	cfg.MinAvailMemPct = t.minAvailPct(keyMinAvailMem, memTotal, 50)
	cfg.MinAvailSwapPct = t.minAvailPct(keyMinAvailSwap, swapTotal, 100)
	cfg.MaxParallel = t.config.GetInt(keyMaxParallel)
	cfg.Env = t.config.GetString(keyEnv)
	cfg.SplitAction = t.config.GetString(keySplitAction)
	cfg.SSHKey = t.config.GetString(keySSHKey)
	cfg.PRKey = t.config.GetString(keyPRKey)

	if cfg.MaxParallel == 0 {
		cfg.MaxParallel = runtime.NumCPU()
	}
	if cfg.MaxParallel < MinMaxParallel {
		cfg.MaxParallel = MinMaxParallel
	}

	for _, s := range t.config.SectionStrings() {
		if !strings.HasPrefix(s, "hook#") {
			continue
		}
		t.log.Tracef("analyse config: %s", s)
		hook := node.Hook{Name: s[5:]}
		if hook.Name == "" {
			t.log.Debugf("skip empty hook name for %s", s)
			continue
		}
		hook.Events = t.config.GetStrings(key.New(s, "events"))
		if len(hook.Events) == 0 {
			t.log.Debugf("skip empty hook events for %s", s)
			continue
		}
		hook.Command = t.config.GetStrings(key.New(s, "command"))
		if len(hook.Command) == 0 {
			t.log.Debugf("skip empty hook command for %s", s)
			continue
		}
		cfg.Hooks = append(cfg.Hooks, hook)
		t.log.Tracef("hook %s: %#v", hook.Name, hook)
	}

	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		t.log.Warnf("load node config: %s", err)
	} else {
		for _, e := range node.Schedules() {
			cfg.Schedules = append(cfg.Schedules, e.Config)
		}
		cfg.Collector = node.CollectorRawConfig().AsConfig()
	}

	return cfg
}

// minAvailPct returns the minimum available share of memory or swap a
// keyword sets, as the percentage of total, in bytes, the node stats are
// compared to.
//
// A size is bounded by limit, as v2 did: a size beyond it is no reasonable
// minimum for the whole, and would hold the node overloaded. A whole of
// unknown size, or none, as a node without swap, sets no minimum: there is
// nothing to fall short of. A value that is no share is reported, and sets
// no minimum.
func (t *Manager) minAvailPct(k key.T, total int64, limit int) int {
	v, err := t.config.Eval(k)
	if err != nil {
		t.log.Warnf("%s: %s: no minimum is set", k, err)
		return 0
	}
	share, ok := v.(sizeconv.Share)
	if !ok {
		return 0
	}
	return share.PercentOf(total, limit)
}

// memTotals returns the memory and the swap of the node, in bytes, and zero
// where they can not be read.
func memTotals() (mem, swap int64) {
	if runtime.GOOS != "linux" {
		return 0, 0
	}
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return 0, 0
	}
	info, err := fs.Meminfo()
	if err != nil {
		return 0, 0
	}
	if info.MemTotal != nil {
		mem = int64(*info.MemTotal) * 1024
	}
	if info.SwapTotal != nil {
		swap = int64(*info.SwapTotal) * 1024
	}
	return mem, swap
}
