package node

import "time"

type (
	Config struct {
		Env                    string        `json:"env"`
		MaintenanceGracePeriod time.Duration `json:"maintenance_grace_period"`
		MaxParallel            int           `json:"max_parallel"`
		ReadyPeriod            time.Duration `json:"ready_period"`
		RejoinGracePeriod      time.Duration `json:"rejoin_grace_period"`
		SplitAction            string        `json:"split_action"`
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
		"max_parallel":             t.MaxParallel,
		"ready_period":             t.ReadyPeriod,
		"rejoin_grace_period":      t.RejoinGracePeriod,
		"split_action":             t.SplitAction,
	}
}
