package ox

import (
	// Necessary to use go:embed
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/version"
)

var (
	colorFlag    string
	selectorFlag string
	serverFlag   string
	quietFlag    bool

	//go:embed bash_completion.sh
	bashCompletionFunction string

	root = &cobra.Command{
		Use:                    filepath.Base(os.Args[0]),
		Short:                  "the opensvc cluster management command",
		SilenceUsage:           true,
		SilenceErrors:          false,
		ValidArgsFunction:      validArgs,
		BashCompletionFunction: bashCompletionFunction,
		Version:                version.Version(),
	}
)

func validArgs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return listObjectPaths(), cobra.ShellCompDirectiveNoFileComp
}

func contextSuffix() string {
	s := env.Context()
	if s == "" {
		return ""
	}
	return "." + s
}

func listObjectPaths() []string {
	if b, err := os.ReadFile(filepath.Join(rawconfig.Paths.Var, "list.objects"+contextSuffix())); err == nil {
		return strings.Fields(string(b))
	}
	return nil
}

func listNodes() []string {
	if b, err := os.ReadFile(filepath.Join(rawconfig.Paths.Var, "list.nodes"+contextSuffix())); err == nil {
		return strings.Fields(string(b))
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
	if len(args) == 0 {
		args = []string{"tui"}
		root.SetArgs(args)
	}

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

	if err != nil || lookupArgs[0] == "-" {
		// command not found... try with args[1] as a selector.
		if len(lookupArgs) > 0 {
			selectorFlag = lookupArgs[0]
			subsystem := guessSubsystem(selectorFlag)
			args := append([]string{}, cobraArgs...)
			args = append(args, subsystem)
			args = append(args, "-s", selectorFlag)
			if len(lookupArgs[1:]) == 0 {
				args = append(args, "tui")
			} else {
				args = append(args, lookupArgs[1:]...)
			}
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

func init() {
	root.PersistentFlags().StringVar(&colorFlag, "color", "auto", "Output colorization yes|no|auto.")
	root.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "do not display logs on the console")
	root.PersistentFlags().StringVar(&serverFlag, "server", "", "URI of the opensvc api server.")
	root.PersistentFlags().StringVarP(&selectorFlag, "selector", "s", "", "object selector")
	root.PersistentFlags().Lookup("selector").Hidden = true
	root.AddCommand(newCmdTUI("svc"))
}

func guessSubsystem(s string) string {
	if p, err := naming.ParsePath(s); err == nil {
		return p.Kind.String()
	}
	return "all"
}

// mergeSelector returns the selector from argv[1], or falls back to
// the selector passed by the -s flag.
func mergeSelector(subsysSelector string, kind string, deft string) string {
	switch {
	case selectorFlag == "-":
		return commoncmd.SelectorFromStdin()
	case selectorFlag != "":
		return selectorFlag
	case subsysSelector != "":
		return fmt.Sprintf("%s+*/%s/*", subsysSelector, kind)
	case deft != "":
		return fmt.Sprintf("%s+*/%s/*", deft, kind)
	default:
		return fmt.Sprintf("*/%s/*", kind)
	}
}
