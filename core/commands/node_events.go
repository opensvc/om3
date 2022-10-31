package commands

import "opensvc.com/opensvc/core/entrypoints"

type (
	CmdNodeEvents struct {
		OptsGlobal
	}
)

func (t *CmdNodeEvents) Run() error {
	e := entrypoints.Events{
		Format: t.Format,
		Color:  t.Color,
		Server: t.Server,
	}
	return e.Do()
}
