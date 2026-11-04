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

const (
	// DefaultTimeout bounds a request to the collector, unless
	// collector.timeout says otherwise.
	DefaultTimeout = 5 * time.Second

	// MinTimeout and MaxTimeout bound collector.timeout.
	MinTimeout = time.Second
	MaxTimeout = 20 * time.Second
)

var (
	ErrConfig       = errors.New("collector is not configured")
	ErrUnregistered = errors.New("this node is not registered. try 'om node register'")
)

// BoundTimeout returns the collector request timeout d, the
// collector.timeout value, within MinTimeout and MaxTimeout, or
// DefaultTimeout when not set.
func BoundTimeout(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultTimeout
	}
	return min(max(d, MinTimeout), MaxTimeout)
}

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
