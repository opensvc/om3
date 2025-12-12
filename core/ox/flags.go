package ox

import (
	// Necessary to use go:embed
	_ "embed"

	"github.com/spf13/pflag"

	commands "github.com/opensvc/om3/v3/core/oxcmd"
)

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	flagSet.StringVar(&p.Color, "color", "auto", "output colorization yes|no|auto")
	flagSet.StringVarP(&p.Output, "output", "o", "auto", "output format json|flat|auto|tab=<header>:<jsonpath>,...")
	flagSet.StringVarP(&p.ObjectSelector, "selector", "s", "", "execute on a list of objects")

}

func addFlagObject(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "selector", "s", "", "execute on a list of objects")
}
