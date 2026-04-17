package commoncmd

import (
	"fmt"

	"github.com/spf13/pflag"
)

// rawStringSlice implements pflag.Value for []string without comma-splitting
type rawStringSlice struct {
	values *[]string
}

func (r *rawStringSlice) String() string {
	if r.values == nil {
		return "[]"
	}
	return fmt.Sprintf("%v", *r.values)
}

func (r *rawStringSlice) Set(val string) error {
	*r.values = append(*r.values, val)
	return nil
}

func (r *rawStringSlice) Type() string {
	return "stringSlice"
}

func RawStringSliceVarP(flags *pflag.FlagSet, p *[]string, name, shorthand string, value []string, usage string) {
	*p = value
	flags.VarP(&rawStringSlice{values: p}, name, shorthand, usage)
}

func RawStringSliceVar(flags *pflag.FlagSet, p *[]string, name string, value []string, usage string) {
	*p = value
	flags.Var(&rawStringSlice{values: p}, name, usage)
}
