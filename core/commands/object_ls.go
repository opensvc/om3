package commands

import (
	"opensvc.com/opensvc/core/entrypoints"
)

type (
	CmdObjectLs struct {
		OptsGlobal
	}
)

func (t *CmdObjectLs) Run(selector, kind string) error {
	return entrypoints.List{
		ObjectSelector: mergeSelector(selector, t.ObjectSelector, kind, "**"),
		Format:         t.Format,
		Color:          t.Color,
		Local:          t.Local,
		Server:         t.Server,
	}.Do()
}
