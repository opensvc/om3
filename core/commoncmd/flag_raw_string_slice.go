package commoncmd

import (
	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/util/flagvalue"
)

// RawStringSliceVarP declares a repeatable option whose values are taken
// whole. It lives in util/flagvalue, which the command sets that cannot import
// this package reach too.
func RawStringSliceVarP(flags *pflag.FlagSet, p *[]string, name, shorthand string, value []string, usage string) {
	flagvalue.RawStringSliceVarP(flags, p, name, shorthand, value, usage)
}

// RawStringSliceVar declares a repeatable option whose values are taken whole.
func RawStringSliceVar(flags *pflag.FlagSet, p *[]string, name string, value []string, usage string) {
	flagvalue.RawStringSliceVar(flags, p, name, value, usage)
}
