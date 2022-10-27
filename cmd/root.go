package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/driverdb"
	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/core/osagentservice"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/logging"
	"opensvc.com/opensvc/util/xsession"

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
	nodeFlag     string
	debugFlag    bool
	callerFlag   bool
)

var root = &cobra.Command{
	Use:               "opensvc",
	Short:             "Manage the opensvc cluster infrastructure and its deployed services.",
	PersistentPreRunE: persistentPreRunE,
	SilenceUsage:      true,
	SilenceErrors:     false,
	ValidArgsFunction: validArgs,
	BashCompletionFunction: `__opensvc_handle_word()
{
    [ $cword -gt 1 ] && [ ! -z "${words[1]}" ] && ! __opensvc_contains_word ${words[1]} svc vol sec cfg usr ccfg nscfg all completion create daemon monitor help && {
        words[1]="all"
    }
    ___opensvc_handle_word
}

___opensvc_handle_word()
{ 
    if [[ $c -ge $cword ]]; then
        __opensvc_handle_reply
        return
    fi
    __opensvc_debug "${FUNCNAME[0]}: c is $c words[c] is ${words[c]}"
    if [[ "${words[c]}" == -* ]]; then
        __opensvc_handle_flag
    elif __opensvc_contains_word "${words[c]}" "${commands[@]}"; then
        __opensvc_handle_command
    elif [[ $c -eq 0 ]]; then
        __opensvc_handle_command 
    elif __opensvc_contains_word "${words[c]}" "${command_aliases[@]}"; then
        # aliashash variable is an associative array which is only supported in bash > 3.
        if [[ -z "${BASH_VERSION}" || "${BASH_VERSINFO[0]}" -gt 3 ]]; then
            words[c]=${aliashash[${words[c]}]}
            __opensvc_handle_command
        else
            __opensvc_handle_noun
        fi
    else
        __opensvc_handle_noun
    fi
    __opensvc_handle_word
}
`,
}

func validArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return listObjectPaths(), cobra.ShellCompDirectiveNoFileComp
}

func listObjectPaths() []string {
	if b, err := os.ReadFile(filepath.Join(rawconfig.Paths.Var, "list.services")); err == nil {
		return strings.Fields(string(b))
	}
	return nil
}

func listNodes() []string {
	if b, err := os.ReadFile(filepath.Join(rawconfig.Paths.Var, "list.nodes")); err == nil {
		return strings.Fields(string(b))
	}
	return nil
}

func configureLogger() {
	initLogger()
	if colorLogFlag == "no" {
		logging.DisableDefaultConsoleWriterColor()
	}
	if debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	if callerFlag {
		log.Logger = log.Logger.With().Caller().Logger()
	}
}

func initLogger() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFieldName = "t"
	zerolog.LevelFieldName = "l"
	zerolog.MessageFieldName = "m"

	l := logging.Configure(logging.Config{
		ConsoleLoggingEnabled: debugFlag || daemonRestartForeground || daemonStartForeground,
		EncodeLogsAsJSON:      true,
		FileLoggingEnabled:    true,
		Directory:             rawconfig.Paths.Log,
		Filename:              "node.log",
		MaxSize:               5,
		MaxBackups:            1,
		MaxAge:                30,
	}).
		With().
		Str("n", hostname.Hostname()).
		Str("sid", xsession.ID).
		Logger()
	log.Logger = l
}

func persistentPreRunE(_ *cobra.Command, _ []string) error {
	logging.WithCaller = callerFlag
	if err := hostname.Error(); err != nil {
		return err
	}
	configureLogger()
	if env.HasDaemonOrigin() {
		if err := osagentservice.Join(); err != nil {
			log.Logger.Debug().Err(err).Msg("")
		}
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the root command.
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
	_, _, err := root.Find(lookupArgs)

	if err != nil {
		// command not found... try with args[1] as a selector.
		if len(lookupArgs) > 0 {
			selectorFlag = lookupArgs[0]
			subsystem := guessSubsystem(selectorFlag)
			args := append([]string{}, cobraArgs...)
			args = append(args, subsystem)
			args = append(args, lookupArgs[1:]...)
			root.SetArgs(args)
			cobra.CompDebug(fmt.Sprintf("modified args: %s\n", args), false)
		}
	}
}

// ExecuteArgs parses args and executes the cobra command.
// Example:
//
//	ExecuteArgs([]string{"mysvc*", "ls"})
func ExecuteArgs(args []string) {
	driverdb.Load()
	setExecuteArgs(args)
	if err := root.Execute(); err != nil {
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
	root.PersistentFlags().StringVar(&configFlag, "config", "", "config file (default \"$HOME/.opensvc.yaml\")")
	root.PersistentFlags().StringVar(&colorFlag, "color", "auto", "output colorization yes|no|auto")
	root.PersistentFlags().StringVar(&colorLogFlag, "colorlog", "auto", "log output colorization yes|no|auto")
	root.PersistentFlags().StringVar(&formatFlag, "format", "auto", "output format json|flat|auto")
	root.PersistentFlags().StringVar(&serverFlag, "server", "", "uri of the opensvc api server. scheme raw|https")
	root.PersistentFlags().BoolVar(&debugFlag, "debug", false, "show debug log")
	root.PersistentFlags().BoolVar(&callerFlag, "caller", false, "show caller <file>:<line> in logs")
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
