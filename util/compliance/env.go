package compliance

import "opensvc.com/opensvc/core/rawconfig"

type (
	// Envs is indexed by module name
	Envs map[string][]string
)

func (t Envs) Render() string {
	buff := ""
	for mod, env := range t {
		buff += rawconfig.Node.Colorize.Bold(mod) + "\n"
		for _, line := range env {
			buff += "  " + line + "\n"
		}
	}
	return buff
}
