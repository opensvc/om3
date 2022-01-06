package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemon"
)

var daemonRunCmd = &cobra.Command{
	Use:   "run",
	Short: "run the daemon",
	RunE:  daemonRunCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonRunCmd)
}

func runDaemon() error {
	main, err := daemon.RunDaemon()
	if err != nil {
		log.Logger.Error().Err(err).Msg("RunDaemon")
		return err
	}
	main.WaitDone()
	return nil
}

func daemonRunCmdRun(_ *cobra.Command, _ []string) error {
	return runDaemon()
}
