package om

import (
	// Necessary to use go:embed
	_ "embed"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/core/commoncmd"
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

func addFlagsGlobalObjectSelector(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVarP(&p.ObjectSelector, "service", "", "", "execute on a list of objects")
	flagSet.StringVarP(&p.ObjectSelector, "selector", "s", "", "execute on a list of objects")
	flagSet.MarkHidden("service")
}

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	addFlagsGlobalLocal(flagSet, p)
	addFlagsGlobalQuiet(flagSet, p)
	addFlagsGlobalDebug(flagSet, p)
	commoncmd.FlagColor(flagSet, &p.Color)
	commoncmd.FlagOutput(flagSet, &p.Output)
	commoncmd.FlagObjectSelector(flagSet, &p.ObjectSelector)
}

func addFlagMonitor(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "monitor", "m", false, "refresh only the monitored resources in the cached instance status data")
}

func addFlagObject(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "service", "", "", "execute on a list of objects")
	flagSet.StringVarP(p, "selector", "s", "", "execute on a list of objects")
	flagSet.MarkHidden("service")

}
