package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
)

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "run the daemon",
	RunE:  daemonRunCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonRunCmd)
}

func daemonRunCmdRun(_ *cobra.Command, _ []string) error {
	if err := daemoncli.Run(); err != nil {
		log.Logger.Error().Err(err).Msg("daemoncli.Run")
		return err
	}
	return nil
}
