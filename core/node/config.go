package node

import "time"

type (
	Config struct {
		MaintenanceGracePeriod time.Duration
		ReadyPeriod            time.Duration
		RejoinGracePeriod      time.Duration
	}
)

func (t *Config) DeepCopy() *Config {
	var data Config = *t
	return &data

}
