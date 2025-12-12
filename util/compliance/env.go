package compliance

import "github.com/opensvc/om3/v3/core/rawconfig"

type (
	// Envs is indexed by module name
	Envs map[string][]string
)

func (t Envs) Render() string {
	buff := ""
	for mod, env := range t {
		buff += rawconfig.Colorize.Bold(mod) + "\n"
		for _, line := range env {
			buff += "  " + line + "\n"
		}
	}
	return buff
}
