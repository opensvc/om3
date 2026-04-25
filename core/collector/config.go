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

		// Hidden fields
		Password string `json:"-"`
	}
)

var (
	ErrConfig       = errors.New("collector is not configured: empty configuration keyword node.dbopensvc")
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
