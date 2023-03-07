package node

import "time"

type (
	Config struct {
		Env                    string        `json:"env"`
		MaintenanceGracePeriod time.Duration `json:"maintenance_grace_period"`
		ReadyPeriod            time.Duration `json:"ready_period"`
		RejoinGracePeriod      time.Duration `json:"rejoin_grace_period"`
		SplitAction            string        `json:"split_action"`
	}
)

func (t *Config) DeepCopy() *Config {
	var data Config = *t
	return &data

}
