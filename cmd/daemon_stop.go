package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
)

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the daemon",
	RunE:  daemonStopCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonStopCmd)
}

func daemonStopCmdRun(_ *cobra.Command, _ []string) error {
	if err := daemoncli.Stop(); err != nil {
		log.Logger.Error().Err(err).Msg("daemoncli.Stop")
		return err
	}
	return nil
}
