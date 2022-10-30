package commands

import "opensvc.com/opensvc/core/entrypoints"

type (
	CmdDaemonStats struct {
		OptsGlobal
	}
)

func (t *CmdDaemonStats) Run() error {
	return entrypoints.DaemonStats{
		Format: t.Format,
		Color:  t.Color,
		Server: t.Server,
	}.Do()
}
