package commoncmd

import (
	"github.com/spf13/pflag"
)

// AddFlagsGlobal adds the global flags to the flag set.
// This is the common implementation used by both om and ox.
func AddFlagsGlobal(flags *pflag.FlagSet, p *OptsGlobal) {
	FlagColor(flags, &p.Color)
	FlagOutput(flags, &p.Output)
	FlagObjectSelector(flags, &p.ObjectSelector)
	FlagIgnoreNotFound(flags, &p.IgnoreNotFound)
}

// AddFlagsGlobalWithColorOutput adds color and output flags
func AddFlagsGlobalWithColorOutput(flags *pflag.FlagSet, color, output *string) {
	FlagColor(flags, color)
	FlagOutput(flags, output)
}

// AddFlagsGlobalWithSelector adds object selector and ignore not found flags
func AddFlagsGlobalWithSelector(flags *pflag.FlagSet, objectSelector *string, ignoreNotFound *bool) {
	FlagObjectSelector(flags, objectSelector)
	FlagIgnoreNotFound(flags, ignoreNotFound)
}