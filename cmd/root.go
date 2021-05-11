package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/osagentservice"
	"opensvc.com/opensvc/util/logging"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	configFlag   string
	colorFlag    string
	formatFlag   string
	selectorFlag string
	serverFlag   string
	debugFlag    bool
)

var rootCmd = &cobra.Command{
	Use:               "opensvc",
	Short:             "Manage the opensvc cluster infrastructure and its deployed services.",
	PersistentPreRunE: persistentPreRunE,
}

func persistentPreRunE(_ *cobra.Command, _ []string) error {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.TimestampFieldName = "t"
	zerolog.LevelFieldName = "l"
	zerolog.MessageFieldName = "m"

	if debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	l := logging.Configure(logging.Config{
		ConsoleLoggingEnabled: true,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             config.Node.Paths.Log,
		Filename:              "node.log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Str("n", config.Node.Hostname).
		Str("sid", config.SessionID).
		Logger()
	log.Logger = l

	if config.HasDaemonOrigin() {
		if err := osagentservice.Join(); err != nil {
			l.Debug().Err(err).Msg("")
		}
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	_, _, err := rootCmd.Find(os.Args[1:])

	if err != nil {
		// command not found... try look in args[1] as a selector
		if len(os.Args) > 1 {
			selectorFlag = os.Args[1]
			args := append([]string{"svc"}, os.Args[2:]...)
			rootCmd.SetArgs(args)
		}
	}

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&configFlag, "config", "", "config file (default \"$HOME/.opensvc.yaml\")")
	rootCmd.PersistentFlags().StringVar(&colorFlag, "color", "auto", "output colorization yes|no|auto")
	rootCmd.PersistentFlags().StringVar(&formatFlag, "format", "auto", "output format json|flat|auto")
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "uri of the opensvc api server. scheme raw|https")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "show debug log")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if configFlag != "" {
		// Use config file from the flag.
		viper.SetConfigFile(configFlag)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".opensvc" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".opensvc")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

// mergeSelector returns the selector from argv[1], or falls back to
// the selector passed by the -s flag.
func mergeSelector(subsysSelector string, kind string, deft string) string {
	switch {
	case selectorFlag != "":
		return selectorFlag
	case subsysSelector != "":
		return fmt.Sprintf("%s+*/%s/*", subsysSelector, kind)
	}
	return fmt.Sprintf("%s+*/%s/*", deft, kind)
}
