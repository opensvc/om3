package ox

import (
	// Necessary to use go:embed
	_ "embed"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/core/commoncmd"
	commands "github.com/opensvc/om3/v3/core/oxcmd"
)

func addFlagsGlobal(flagSet *pflag.FlagSet, p *commands.OptsGlobal) {
	commoncmd.AddFlagsGlobal(flagSet, &p.OptsGlobal)
}

func addFlagObject(flagSet *pflag.FlagSet, p *string) {
	flagSet.StringVarP(p, "selector", "s", "", "execute on a list of objects")
}
