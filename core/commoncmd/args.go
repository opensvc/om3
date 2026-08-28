package commoncmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// discardValue stands for a real flag value in the throwaway flag set
// firstNonFlag() parses with. Parsing must not write the values cobra is
// about to parse itself: a repeated parse would append twice to the slice
// flags.
type discardValue struct {
	typ string
}

func (t discardValue) String() string   { return "" }
func (t discardValue) Set(string) error { return nil }
func (t discardValue) Type() string     { return t.typ }

// ValidateArgs returns an error when args name a command that does not exist.
//
// cobra walks the command chain and, when the deepest command it reaches has
// no Run function, prints that command help and returns no error, silently
// dropping whatever was typed after it. A stale or mistyped command path
// exits 0 that way, which is how the daemon scheduler and the api exec kept
// reporting successful runs of commands renamed under their hard-coded argv.
func ValidateArgs(root *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if strings.HasPrefix(args[0], "__complete") {
		// shell completion: cobra answers with candidates, it does not run
		return nil
	}
	cmd, rest, err := root.Find(args)
	if err != nil || cmd == nil || cmd.Runnable() {
		// an unknown first word is reported by cobra itself
		return nil
	}
	name, ok := firstNonFlag(cmd, rest)
	if !ok {
		// no command name left: the user asked for the command list
		return nil
	}
	return fmt.Errorf("unknown command %q for %q", name, cmd.CommandPath())
}

// firstNonFlag returns the first positional argument left in args once the
// flags cmd accepts are stripped.
func firstNonFlag(cmd *cobra.Command, args []string) (string, bool) {
	fs := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = true
	fs.SetOutput(io.Discard)

	add := func(f *pflag.Flag) {
		if fs.Lookup(f.Name) != nil {
			return
		}
		shorthand := f.Shorthand
		if shorthand != "" && fs.ShorthandLookup(shorthand) != nil {
			shorthand = ""
		}
		fs.AddFlag(&pflag.Flag{
			Name:      f.Name,
			Shorthand: shorthand,
			Value:     discardValue{typ: f.Value.Type()},
			DefValue:  f.DefValue,
			// carried over so the valueless flags don't eat the next word
			NoOptDefVal: f.NoOptDefVal,
		})
	}
	// InheritedFlags() and LocalFlags() both merge the parents persistent
	// flags into cmd.Flags() on the way, which Flags() alone does not do
	// before the command runs.
	cmd.InheritedFlags().VisitAll(add)
	cmd.LocalFlags().VisitAll(add)
	cmd.Flags().VisitAll(add)

	if err := fs.Parse(args); err != nil {
		// a flag error is cobra's to report
		return "", false
	}
	if rest := fs.Args(); len(rest) > 0 {
		return rest[0], true
	}
	return "", false
}
