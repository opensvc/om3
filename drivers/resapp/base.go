package resapp

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/envprovider"
	"os"
	"time"
)

// BaseT is the app base driver structure
type BaseT struct {
	resource.T
	RetCodes     string         `json:"retcodes"`
	Path         path.T         `json:"path"`
	Nodes        []string       `json:"nodes"`
	SecretsEnv   []string       `json:"secret_environment"`
	ConfigsEnv   []string       `json:"configs_environment"`
	Env          []string       `json:"environment"`
	Timeout      *time.Duration `json:"timeout"`
	StartTimeout *time.Duration `json:"start_timeout"`
	StopTimeout  *time.Duration `json:"stop_timeout"`
	Umask        *os.FileMode   `json:"umask"`
}

func (t BaseT) getEnv() (env []string, err error) {
	var tempEnv []string
	// TODO update: OPENSVC_RID, OPENSVC_NAME, OPENSVC_KIND, OPENSVC_ID, OPENSVC_NAMESPACE
	if len(t.Env) > 0 {
		env = append(env, t.Env...)
	}
	if tempEnv, err = envprovider.From(t.ConfigsEnv, t.Path.Namespace, "cfg"); err != nil {
		t.Log().Error().Err(err).Msgf("unable to retrieve env from configs_environment: '%v'", t.ConfigsEnv)
		return nil, err
	} else {
		env = append(env, tempEnv...)
	}
	if tempEnv, err = envprovider.From(t.SecretsEnv, t.Path.Namespace, "sec"); err != nil {
		t.Log().Error().Err(err).Msgf("unable to retrieve env from secrets_environment: '%v'", t.SecretsEnv)
		return nil, err
	} else {
		env = append(env, tempEnv...)
	}
	return env, nil
}
