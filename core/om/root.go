package om

import (
	// Necessary to use go:embed
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/osagentservice"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/fcache"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/logging"
	"github.com/opensvc/om3/util/render"
	"github.com/opensvc/om3/util/version"
	"github.com/opensvc/om3/util/xsession"
)

var (
	colorFlag    string
	nodeFlag     string
	selectorFlag string

	callerFlag     bool
	debugFlag      bool
	foregroundFlag bool
	quietFlag      bool
	versionFlag    bool

	//go:embed bash_completion.sh
	bashCompletionFunction string

	root = &cobra.Command{
		Use:                    filepath.Base(os.Args[0]),
		Short:                  "the opensvc cluster management command",
		PersistentPreRunE:      persistentPreRunE,
		SilenceUsage:           true,
		SilenceErrors:          false,
		ValidArgsFunction:      validArgs,
		BashCompletionFunction: bashCompletionFunction,
		Version:                version.Version(),
	}
)

func validArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	//return listObjectPaths(), cobra.ShellCompDirectiveNoFileComp
	all := listObjectPaths()
	if toComplete == "" {
		return all, cobra.ShellCompDirectiveNoFileComp
	}
	l := make([]string, 0)
	for _, candidate := range all {
		if strings.HasPrefix(candidate, toComplete) {
			l = append(l, candidate)
		}
	}
	return l, cobra.ShellCompDirectiveNoFileComp
}

func listObjectPaths() []string {
	if b, err := os.ReadFile(filepath.Join(rawconfig.Paths.Var, "list.objects")); err == nil {
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

func configureLogger() error {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	level := "info"
	if debugFlag {
		level = "debug"
	}
	err := logging.Configure(logging.Config{
		WithConsoleLog: !quietFlag || debugFlag || foregroundFlag,
		WithColor:      colorFlag != "no",
		WithCaller:     callerFlag,
		Level:          level,
	})
	if err != nil {
		return err
	}
	log.Logger = log.Logger.With().
		Str("node", hostname.Hostname()).
		Str("version", version.Version()).
		Stringer("sid", xsession.ID).
		Logger()
	if requestID := os.Getenv("OSVC_REQUEST_ID"); requestID != "" {
		log.Logger = log.Logger.With().Str("request_id", requestID).Logger()
	}
	return nil
}

func persistentPreRunE(cmd *cobra.Command, _ []string) error {
	if flag := cmd.Flags().Lookup("quiet"); flag != nil && flag.Value.String() == "true" {
		quietFlag = true
	}
	if flag := cmd.Flags().Lookup("foreground"); flag != nil && flag.Value.String() == "true" {
		foregroundFlag = true
	}
	if flag := cmd.Flags().Lookup("debug"); flag != nil && flag.Value.String() == "true" {
		debugFlag = true
	}
	if flag := cmd.Flags().Lookup("color"); flag != nil {
		colorFlag = flag.Value.String()
		render.SetColor(colorFlag)
	}
	logging.WithCaller = callerFlag
	if err := hostname.Error(); err != nil {
		return err
	}
	if err := configureLogger(); err != nil {
		return err
	}

	// Ignore errors so non-root can use om until it needs the root privilege.
	// For example, om svc doc doesn't need root privilege.
	_, _ = object.SetClusterConfig()

	if env.HasDaemonOrigin() {
		if err := osagentservice.Join(); err != nil {
			log.Logger.Debug().Err(err).Send()
		}
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the root command.
func Execute() {
	ExecuteArgs(os.Args[1:])
	fcache.PurgeCache()
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
	} else if len(args) > 0 {
		//
		// Example:
		//   args = [test/svc/s1 pri]
		//   => lookupArgs = [test/svc/s1 pri]
		//   => cobraArgs = []
		//
		lookupArgs = args
		cobraArgs = []string{}
	} else {
		return
	}

	_, _, err := root.Find(lookupArgs)

	if err != nil || lookupArgs[0] == "-" {
		// command not found... try with args[1] as a selector.
		if len(lookupArgs) > 0 {
			selectorFlag = lookupArgs[0]
			subsystem := guessSubsystem(selectorFlag)
			args := append([]string{}, cobraArgs...)
			args = append(args, subsystem)
			args = append(args, "-s", selectorFlag)
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
	type exitcoder interface {
		ExitCode() int
	}
	var xc int
	var xerr exitcoder
	setExecuteArgs(args)
	if err := root.Execute(); err != nil {
		if errors.As(err, &xerr) {
			xc = xerr.ExitCode()
		} else {
			xc = 1
		}
		os.Exit(xc)
	}
}

func guessSubsystem(s string) string {
	if p, err := naming.ParsePath(s); err == nil {
		return p.Kind.String()
	}
	return "all"
}

func init() {
	root.PersistentFlags().StringVar(&colorFlag, "color", "auto", "output colorization (yes|no|auto)")
	root.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "do not display logs on the console")
	root.PersistentFlags().BoolVar(&debugFlag, "debug", false, "display logs at debug level")
	root.PersistentFlags().BoolVar(&callerFlag, "caller", false, "show the caller file and linenum in logs")
	root.PersistentFlags().StringVarP(&selectorFlag, "selector", "s", "", "object selector")
	root.PersistentFlags().Lookup("selector").Hidden = true
	root.RegisterFlagCompletionFunc("selector", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return validArgs(cmd, args, toComplete)
	})
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
