// Package flagvalue holds the option types the standard library and pflag do
// not provide, for the command sets that need them.
package flagvalue

import (
	"fmt"

	"github.com/spf13/pflag"
)

// rawStringSlice implements pflag.Value for []string without comma-splitting.
//
// pflag's own string slice reads a comma as a separator, which is wrong for a
// value that holds commas of its own: a mapping is written
// "<hba>:<tgt>,<tgt>", and splitting it there would turn one mapping into two
// halves of one.
type rawStringSlice struct {
	values *[]string
}

func (t *rawStringSlice) String() string {
	if t.values == nil {
		return "[]"
	}
	return fmt.Sprintf("%v", *t.values)
}

func (t *rawStringSlice) Set(val string) error {
	*t.values = append(*t.values, val)
	return nil
}

func (t *rawStringSlice) Type() string {
	return "stringSlice"
}

// Append, Replace and GetSlice make this a pflag.SliceValue, so a reader can
// take the values back as they were given rather than through the string form,
// which is where the comma would come back to split them.
func (t *rawStringSlice) Append(val string) error {
	*t.values = append(*t.values, val)
	return nil
}

func (t *rawStringSlice) Replace(values []string) error {
	*t.values = values
	return nil
}

func (t *rawStringSlice) GetSlice() []string {
	if t.values == nil {
		return nil
	}
	return *t.values
}

// RawStringSliceVarP declares an option that can be repeated, and whose values
// are taken whole.
func RawStringSliceVarP(flags *pflag.FlagSet, p *[]string, name, shorthand string, value []string, usage string) {
	*p = value
	flags.VarP(&rawStringSlice{values: p}, name, shorthand, usage)
}

// RawStringSliceVar declares an option that can be repeated, and whose values
// are taken whole.
func RawStringSliceVar(flags *pflag.FlagSet, p *[]string, name string, value []string, usage string) {
	RawStringSliceVarP(flags, p, name, "", value, usage)
}
