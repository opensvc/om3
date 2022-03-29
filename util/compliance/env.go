package compliance

import (
	"fmt"
	"strings"
)

type (
	// Envs is indexed by module name
	Envs map[string]Env

	// Env is indexed by env var name
	Env map[string]string
)

func (t Envs) Render() string {
	buff := ""
	for mod, env := range t {
		buff += mod + "\n"
		for _, line := range strings.Split(env.Render(), "\n") {
			buff += " " + line + "\n"
		}
	}
	return buff
}

func (t Env) Render() string {
	buff := ""
	for k, v := range t {
		buff += fmt.Sprintf("%s=%s\n", k, v)
	}
	return buff
}
func (t T) GetEnvs(modsets, mods []string) (Envs, error) {
	m := make(Envs)
	data, err := t.GetData(modsets)
	if err != nil {
		return nil, err
	}
	for _, mod := range data.ExpandModules(modsets, mods) {
		vars := NewVars()
		vars.LoadEnv()
		for _, rset := range data.Rsets {
			vars = append(vars, rset.Vars...)
		}
		m[mod] = vars.Env()
	}
	return m, nil
}
