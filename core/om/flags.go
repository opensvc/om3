package om

import (
	// Necessary to use go:embed
	_ "embed"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/core/commoncmd"
	commands "github.com/opensvc/om3/core/omcmd"
)

func hiddenFlagLocal(flagSet *pflag.FlagSet, p *bool) {
	flagLocal(flagSet, p)
	flagSet.Lookup("local").Hidden = true
}

func flagLocal(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "local", false, "inline action on local instance")

}

func flagQuiet(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVarP(p, "quiet", "q", false, "don't print the logs on the console")
}

func flagDebug(flagSet *pflag.FlagSet, p *bool) {
	flagSet.BoolVar(p, "debug", false, "show debug log entries")
}

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagLocal(flagSet, &p.Local)
	flagQuiet(flagSet, &p.Quiet)
	flagDebug(flagSet, &p.Debug)
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
