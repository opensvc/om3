package node

import "time"

type (
	Config struct {
		Env                    string        `json:"env"`
		MaintenanceGracePeriod time.Duration `json:"maintenance_grace_period"`
		MaxParallel            int           `json:"max_parallel"`
		MaxKeySize             int64         `json:"max_key_size"`
		MinAvailMemPct         int           `json:"min_avail_mem_pct"`
		MinAvailSwapPct        int           `json:"min_avail_swap_pct"`
		ReadyPeriod            time.Duration `json:"ready_period"`
		RejoinGracePeriod      time.Duration `json:"rejoin_grace_period"`
		SplitAction            string        `json:"split_action"`
		SSHKey                 string        `json:"sshkey"`
		PRKey                  string        `json:"prkey"`
	}
)

func (t *Config) DeepCopy() *Config {
	var data Config = *t
	return &data

}
