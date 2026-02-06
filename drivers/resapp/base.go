package resapp

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/actioncontext"
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
}

func (t *T) getEnv(ctx context.Context) (env []string, err error) {
	var tempEnv []string
	env = []string{
		"OPENSVC_RID=" + t.RID(),
		"OPENSVC_NAME=" + t.Path.String(),
		"OPENSVC_KIND=" + t.Path.Kind.String(),
		"OPENSVC_ID=" + t.ObjectID.String(),
		"OPENSVC_NAMESPACE=" + t.Path.Namespace,
	}
	if len(t.Env) > 0 {
		env = append(env, t.Env...)
	}
	if tempEnv, err = envprovider.From(t.ConfigsEnv, t.Path.Namespace, "cfg"); err != nil {
		return nil, err
	}
	env = append(env, tempEnv...)
	if tempEnv, err = envprovider.From(t.SecretsEnv, t.Path.Namespace, "sec"); err != nil {
		return nil, err
	}
	env = append(env, tempEnv...)
	env = append(env, actioncontext.Env(ctx)...)
	return env, nil
}
