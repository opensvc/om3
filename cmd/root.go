package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/osagentservice"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/logging"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	configFlag   string
	colorFlag    string
	colorLogFlag string
	formatFlag   string
	selectorFlag string
	serverFlag   string
	debugFlag    bool
)

var rootCmd = &cobra.Command{
	Use:               "opensvc",
	Short:             "Manage the opensvc cluster infrastructure and its deployed services.",
	PersistentPreRunE: persistentPreRunE,
	ValidArgsFunction: validArgs,
}

func validArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return listObjectPaths(), cobra.ShellCompDirectiveNoFileComp
}

func listObjectPaths() []string {
	if b, err := file.ReadAll(filepath.Join(config.Node.Paths.Var, "list.services")); err == nil {
		return strings.Fields(string(b))
	}
	return nil
}

func listNodes() []string {
	if b, err := file.ReadAll(filepath.Join(config.Node.Paths.Var, "list.nodes")); err == nil {
		return strings.Fields(string(b))
	}
	return nil
}

func configureLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.TimestampFieldName = "t"
	zerolog.LevelFieldName = "l"
	zerolog.MessageFieldName = "m"

	if colorLogFlag == "no" {
		logging.DisableDefaultConsoleWriterColor()
	}
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
}

func persistentPreRunE(_ *cobra.Command, _ []string) error {
	configureLogger()
	if config.HasDaemonOrigin() {
		if err := osagentservice.Join(); err != nil {
			log.Logger.Debug().Err(err).Msg("")
		}
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ExecuteArgs(os.Args[1:])
}

func setExecuteArgs(args []string) {
	var lookupArgs, cobraArgs []string
	//
	// Note:
	//   Cobra uses __complete and __completeNoDesc hidden actions
	//
	if len(args) > 0 && strings.HasPrefix(args[0], "__complete") {
		//
		// Example:
		//   args = [__completeNoDesc test/svc/s1 pri]
		//   => lookupArgs = [test/svc/s1 pri]
		//   => cobraArgs = [__completeNoDesc]
		//
		lookupArgs = args[1:]
		cobraArgs = args[0:1]
	} else {
		//
		// Example:
		//   args = [test/svc/s1 pri]
		//   => lookupArgs = [test/svc/s1 pri]
		//   => cobraArgs = []
		//
		lookupArgs = args
		cobraArgs = []string{}
	}
	_, _, err := rootCmd.Find(lookupArgs)

	if err != nil {
		// command not found... try with args[1] as a selector.
		if len(lookupArgs) > 0 {
			selectorFlag = lookupArgs[0]
			subsystem := guessSubsystem(selectorFlag)
			args := append([]string{}, cobraArgs...)
			args = append(args, subsystem)
			args = append(args, lookupArgs[1:]...)
			rootCmd.SetArgs(args)
			cobra.CompDebug(fmt.Sprintf("modified args: %s\n", args), false)
		}
	}
}

// ExecuteArgs parses args and executes the cobra command.
// Example:
//  ExecuteArgs([]string{"mysvc*", "ls"})
func ExecuteArgs(args []string) {
	setExecuteArgs(args)
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func guessSubsystem(s string) string {
	if p, err := path.Parse(s); err == nil {
		return p.Kind.String()
	}
	return "all"
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&configFlag, "config", "", "config file (default \"$HOME/.opensvc.yaml\")")
	rootCmd.PersistentFlags().StringVar(&colorFlag, "color", "auto", "output colorization yes|no|auto")
	rootCmd.PersistentFlags().StringVar(&colorLogFlag, "colorlog", "auto", "log output colorization yes|no|auto")
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
