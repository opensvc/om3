package cmd

import (
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/util/command"
)

var (
	daemonStartCmd = &cobra.Command{
		Use:     "start",
		Short:   "Start the daemon or a daemon thread pointed by '--thread-id'.",
		Aliases: []string{"star"},
		RunE:    daemonStartCmdRun,
	}
	daemonStartForeground bool
)

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonStartCmd.Flags().BoolVarP(
		&daemonStartForeground,
		"foreground",
		"f",
		false,
		"Run the deamon in foreground mode.")

}

func daemonStartCmdRun(_ *cobra.Command, _ []string) error {
	if daemonStartForeground {
		return runDaemon()
	} else {
		args := []string{"daemon", "run"}
		if debugFlag {
			args = append(args, "--debug")
		}
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs(args),
		)
		if err := cmd.Start(); err != nil {
			log.Logger.Error().Err(err).Msg("daemon run")
		}
	}
	return nil
}
