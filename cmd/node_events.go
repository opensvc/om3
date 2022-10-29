package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints"
)

func nodeEventsCmdRun(_ *cobra.Command, _ []string) error {
	e := entrypoints.Events{
		Format: formatFlag,
		Color:  colorFlag,
		Server: serverFlag,
	}
	return e.Do()
}
