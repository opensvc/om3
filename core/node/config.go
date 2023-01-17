package node

import "time"

type (
	Config struct {
		MaintenanceGracePeriod time.Duration `json:"maintenance_grace_period"`
		ReadyPeriod            time.Duration `json:"ready_period"`
		RejoinGracePeriod      time.Duration `json:"rejoin_grace_period"`

		// fields private, no exposed in daemon data
		// json nor events
		secret string
	}
)

func (t *Config) DeepCopy() *Config {
	var data Config = *t
	return &data

}

func (t Config) Secret() string {
	return t.secret
}
func (t *Config) SetSecret(s string) {
	t.secret = s
}
