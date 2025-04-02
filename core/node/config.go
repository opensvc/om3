package node

import "time"

type (
	Config struct {
		Env                    string        `json:"env"`
		MaintenanceGracePeriod time.Duration `json:"maintenance_grace_period"`
		MaxParallel            int           `json:"max_parallel"`
		MinAvailMemPct         int           `json:"min_avail_mem_pct"`
		MinAvailSwapPct        int           `json:"min_avail_swap_pct"`
		ReadyPeriod            time.Duration `json:"ready_period"`
		RejoinGracePeriod      time.Duration `json:"rejoin_grace_period"`
		SplitAction            string        `json:"split_action"`
		SSHKey                 string        `json:"sshkey"`
		PRKey                  string        `json:"pr_key"`
	}
)

func (t *Config) DeepCopy() *Config {
	var data Config = *t
	return &data

}

func (t *Config) Unstructured() map[string]any {
	return map[string]any{
		"env":                      t.Env,
		"maintenance_grace_period": t.MaintenanceGracePeriod,
		"min_avail_mem_pct":        t.MinAvailMemPct,
		"min_avail_swap_pct":       t.MinAvailSwapPct,
		"max_parallel":             t.MaxParallel,
		"ready_period":             t.ReadyPeriod,
		"rejoin_grace_period":      t.RejoinGracePeriod,
		"split_action":             t.SplitAction,
		"sshkey":                   t.SSHKey,
		"pr_key":                   t.PRKey,
	}
}
