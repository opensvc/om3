package om

import (
	// Necessary to use go:embed
	_ "embed"

	"github.com/spf13/pflag"

	commands "github.com/opensvc/om3/core/omcmd"
)

func addFlagsGlobalLocal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.BoolVar(&p.Local, "local", false, "inline action on local instance")
}

func addFlagsGlobalQuiet(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.BoolVarP(&p.Quiet, "quiet", "q", false, "don't print the logs on the console")
}

func addFlagsGlobalDebug(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.BoolVar(&p.Debug, "debug", false, "show debug log entries")
}

func addFlagsGlobalColor(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVar(&p.Color, "color", "auto", "output colorization yes|no|auto")
}

func addFlagsGlobalOutput(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVarP(&p.Output, "output", "o", "auto", "output format json|flat|auto|tab=<header>:<jsonpath>,...")
	flagSet.StringVar(&p.Output, "format", "auto", "output format json|flat|auto|tab=<header>:<jsonpath>,...")
	flagSet.MarkHidden("format")
}

func addFlagsGlobalObjectSelector(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVarP(&p.ObjectSelector, "service", "", "", "execute on a list of objects")
	flagSet.StringVarP(&p.ObjectSelector, "selector", "s", "", "execute on a list of objects")
	flagSet.MarkHidden("service")
}

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	addFlagsGlobalLocal(flagSet, p)
	addFlagsGlobalQuiet(flagSet, p)
	addFlagsGlobalDebug(flagSet, p)
	addFlagsGlobalColor(flagSet, p)
	addFlagsGlobalOutput(flagSet, p)
	addFlagsGlobalObjectSelector(flagSet, p)
}

func addFlagMonitor(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "monitor", "m", false, "refresh only the monitored resources in the cached instance status data")
}

func addFlagObject(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "service", "", "", "execute on a list of objects")
	flagSet.StringVarP(p, "selector", "s", "", "execute on a list of objects")
	flagSet.MarkHidden("service")

}
