package cmd

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
	"opensvc.com/opensvc/util/command"
)

var (
	daemonRestartCmd = &cobra.Command{
		Use:     "restart",
		Short:   "Start the daemon or a daemon thread pointed by '--thread-id'.",
		Aliases: []string{"restart"},
		RunE:    daemonRestartCmdRun,
	}
	daemonRestartForeground bool
)

func init() {
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonRestartCmd.Flags().BoolVarP(
		&daemonRestartForeground,
		"foreground",
		"f",
		false,
		"Restart the daemon in foreground mode.")

}

func daemonRestartCmdRun(_ *cobra.Command, _ []string) error {
	if daemonRestartForeground {
		if err := daemoncli.ReStart(); err != nil {
			log.Logger.Error().Err(err).Msg("daemoncli.Restart")
			os.Exit(1)
		}
	} else {
		args := []string{"daemon", "restart"}
		if debugFlag {
			args = append(args, "--debug")
		}
		args = append(args, "--foreground")
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs(args),
		)
		checker := func() error {
			time.Sleep(60 * time.Millisecond)
			if err := daemoncli.WaitRunning(); err != nil {
				return errors.New("daemon not running")
			}
			return nil
		}
		daemoncli.LockCmdExit(cmd, checker, "daemon restart")
	}
	return nil
}
