package cmd

import (
	"errors"
	"os"
	"runtime/pprof"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/daemon/daemoncli"
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
	cpuprofile            string
)

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonStartCmd.Flags().BoolVarP(&daemonStartForeground, "foreground", "f", false, "Run the daemon in foreground mode.")
	daemonStartCmd.Flags().StringVar(&cpuprofile, "cpuprofile", "", "Dump a cpu pprof in this file on exit.")
}

func daemonStartCmdRun(_ *cobra.Command, _ []string) error {
	cli, err := newClient()
	if err != nil {
		os.Exit(1)
	}
	if daemonStartForeground {
		if cpuprofile != "" {
			f, err := os.Create(cpuprofile)
			if err != nil {
				log.Logger.Fatal().Err(err).Msg("could not create CPU profile")
				os.Exit(1)
			}
			defer f.Close() // error handling omitted for example
			if err := pprof.StartCPUProfile(f); err != nil {
				log.Logger.Fatal().Err(err).Msg("could not start CPU profile")
				os.Exit(1)
			}
			defer pprof.StopCPUProfile()
		}

		if err := daemoncli.New(cli).Start(); err != nil {
			log.Logger.Error().Err(err).Msg("daemoncli.Run")
			os.Exit(1)
		}
	} else {
		args := []string{"daemon", "start", "--foreground"}
		if debugFlag {
			args = append(args, "--debug")
		}
		if serverFlag != "" {
			args = append(args, "--server", serverFlag)
		}
		cmd := command.New(
			command.WithName(os.Args[0]),
			command.WithArgs(args),
		)
		checker := func() error {
			time.Sleep(60 * time.Millisecond)
			if err := daemoncli.New(cli).WaitRunning(); err != nil {
				return errors.New("daemon not running")
			}
			return nil
		}
		daemoncli.LockCmdExit(cmd, checker, "daemon start")
	}
	return nil
}
