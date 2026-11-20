package resapp

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/envprovider"
)

// BaseT is the app base driver structure
type BaseT struct {
	resource.T
	resource.Restart
	RetCodes     string         `json:"retcodes"`
	Path         naming.Path    `json:"path"`
	Nodes        []string       `json:"nodes"`
	SecretsEnv   []string       `json:"secret_environment"`
	ConfigsEnv   []string       `json:"configs_environment"`
	Env          []string       `json:"environment"`
	Timeout      *time.Duration `json:"timeout"`
	StartTimeout *time.Duration `json:"start_timeout"`
	StopTimeout  *time.Duration `json:"stop_timeout"`
	Umask        *os.FileMode   `json:"umask"`
	ObjectID     uuid.UUID      `json:"objectID"`

	// NoNewPrivileges and CapabilityBoundingSet restrict the privileges
	// of the commands, applied by om exec. They are kept as written and
	// converted when a command is built, so a value that is not one fails
	// the command instead of leaving it unrestricted.
	NoNewPrivileges       string `json:"no_new_privileges"`
	CapabilityBoundingSet string `json:"capability_bounding_set"`

	// SystemdProperties are the properties of the transient systemd
	// service unit the start command runs in, as systemd-run -p takes
	// them, kept as written: empty, the command runs in no unit.
	SystemdProperties string `json:"systemd_properties"`
}

func (t *T) getEnv(ctx context.Context, onIgnoreCallback func(err error)) (env []string, err error) {
	var envPath string
	if currentPath := os.Getenv("PATH"); currentPath == "" {
		envPath = t.Path.Initd()
	} else {
		envPath = currentPath + ":" + t.Path.Initd()
	}
	env = []string{
		"PATH=" + envPath,
		"OPENSVC_RID=" + t.RID(),
		"OPENSVC_NAME=" + t.Path.String(),
		"OPENSVC_KIND=" + t.Path.Kind.String(),
		"OPENSVC_ID=" + t.ObjectID.String(),
		"OPENSVC_NAMESPACE=" + t.Path.Namespace,
	}
	if len(t.Env) > 0 {
		l, _, err := datarecv.Environment(t.Env, t.Path.Namespace, func(err error) error {
			// The status action passes no callback: it goes on silently.
			if onIgnoreCallback != nil {
				onIgnoreCallback(err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		env = append(env, l...)
	}
	if tempEnv, err := envprovider.From(t.ConfigsEnv, t.Path.Namespace, "cfg", envprovider.IgnoreExpected(onIgnoreCallback)); err != nil {
		return nil, err
	} else {
		env = append(env, tempEnv...)
	}
	if tempEnv, err := envprovider.From(t.SecretsEnv, t.Path.Namespace, "sec", envprovider.IgnoreExpected(onIgnoreCallback)); err != nil {
		return nil, err
	} else {
		env = append(env, tempEnv...)
	}
	env = append(env, actioncontext.Env(ctx)...)
	return env, nil
}
