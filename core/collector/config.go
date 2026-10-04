package collector

import (
	"errors"
	"time"
)

type (
	Config struct {
		FeederUrl    string        `json:"feeder_url"`
		ServerUrl    string        `json:"server_url"`
		Timeout      time.Duration `json:"timeout"`
		Insecure     bool          `json:"insecure"`
		PingInterval time.Duration `json:"ping_interval"`
		StatusDelay  time.Duration `json:"status_delay"`

		// ActionBatch is the maximum number of instance action begins and
		// ends the collector speaker sends per batch.
		ActionBatch int `json:"action_batch"`

		// ActionLogTimeout bounds the read of the log lines of an ended
		// instance action on the node it ran on.
		ActionLogTimeout time.Duration `json:"action_log_timeout"`

		// Hidden fields
		Password string `json:"-"`
	}
)

var (
	ErrConfig       = errors.New("collector is not configured")
	ErrUnregistered = errors.New("this node is not registered. try 'om node register'")
)

func (t *Config) Equal(o *Config) bool {
	if t == nil && o != nil {
		return false
	}
	if t != nil && o == nil {
		return false
	}
	if t != nil && o != nil && *t != *o {
		return false
	}
	return true
}

func (t *Config) DeepCopy() *Config {
	if t == nil {
		return nil
	}
	n := *t
	return &n
}
