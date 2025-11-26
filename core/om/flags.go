package om

import (
	// Necessary to use go:embed
	_ "embed"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/core/commoncmd"
	commands "github.com/opensvc/om3/core/omcmd"
)

func addFlagsGlobal(flags *pflag.FlagSet, options *commands.OptsGlobal) {
	commoncmd.FlagColor(flags, &options.Color)
	commoncmd.FlagOutput(flags, &options.Output)
	commoncmd.FlagObjectSelector(flags, &options.ObjectSelector)
}

func flagLocal(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "local", false, "inline action on local instance")
}

func flagStonithNode(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "node", "", "the cluster node to fence")
}

func addFlagMonitor(flags *pflag.FlagSet, p *bool) {
	flags.BoolVarP(p, "monitor", "m", false, "refresh only the monitored resources in the cached instance status data")
}

func addFlagObject(flags *pflag.FlagSet, p *string) {
	flags.StringVarP(p, "service", "", "", "execute on a list of objects")
	flags.StringVarP(p, "selector", "s", "", "execute on a list of objects")
	flags.MarkHidden("service")
}

func hiddenFlagLocal(flags *pflag.FlagSet, p *bool) {
	flagLocal(flags, p)
	flags.Lookup("local").Hidden = true
}
