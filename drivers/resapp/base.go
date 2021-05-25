package resapp

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
	"os"
	"time"
)

// BaseT is the app base driver structure
type BaseT struct {
	resource.T
	RetCodes     string         `json:"retcodes"`
	Path         path.T         `json:"path"`
	Nodes        []string       `json:"nodes"`
	SecretEnv    []string       `json:"secret_environment"`
	Timeout      *time.Duration `json:"timeout"`
	ConfigsEnv   []string       `json:"configs_environment"`
	Env          []string       `json:"environment"`
	StartTimeout *time.Duration `json:"start_timeout"`
	StopTimeout  *time.Duration `json:"stop_timeout"`
	Umask        *os.FileMode   `json:"umask"`
}
