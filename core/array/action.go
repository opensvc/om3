package array

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type (
	// Action is one command an array driver answers to, named by the path of
	// words that reaches it: {"add", "disk"} is "om array add disk".
	//
	// A driver declares what it does rather than building a command tree of
	// its own, so the tree is built once, here, and every array is reached
	// the same way. The action reads its options from the Input it is given,
	// not from a variable the command line happened to fill, which is what
	// lets two arrays be driven at once and lets an action be called with no
	// command line at all.
	Action struct {
		// Path is the words leading to this action, outermost first.
		Path []string

		// Short is the one line help of the action.
		Short string

		// Long is the full help of the action, when the short one does not
		// say enough.
		Long string

		// Flags is the options the action reads.
		Flags []Flag

		// Hidden keeps the action out of the help, for one that is answered
		// for the sake of a caller that already writes it and that nobody
		// should be told to start writing.
		Hidden bool

		// Run does the work and returns what to render.
		Run func(ctx context.Context, in Input) (any, error)
	}

	// Actioner is implemented by an array driver declaring its actions.
	//
	// A driver that does not implement it keeps its own command tree, which
	// it parses itself.
	Actioner interface {
		Actions() []Action
	}

	// Input is the options an action was called with.
	Input struct {
		flags *pflag.FlagSet
	}
)

// NewInput returns the input an action reads, for a caller building one
// without a command line, a test among them.
func NewInput(action Action, values map[string]any) (Input, error) {
	flags := pflag.NewFlagSet(strings.Join(action.Path, " "), pflag.ContinueOnError)
	if err := registerFlags(flags, action.Flags); err != nil {
		return Input{}, err
	}
	in := Input{flags: flags}
	for name, value := range values {
		if flags.Lookup(name) == nil {
			return in, fmt.Errorf("option %s is not one of this action", name)
		}
		if err := flags.Set(name, fmt.Sprint(value)); err != nil {
			return in, fmt.Errorf("option %s: %w", name, err)
		}
	}
	return in, nil
}

// String returns the value of a string option, or the zero value when the
// action does not declare it.
func (t Input) String(name string) string {
	s, _ := t.flags.GetString(name)
	return s
}

// Int returns the value of an int option.
func (t Input) Int(name string) int {
	i, _ := t.flags.GetInt(name)
	return i
}

// Bool returns the value of a bool option.
func (t Input) Bool(name string) bool {
	b, _ := t.flags.GetBool(name)
	return b
}

// StringSlice returns the values of an option that can be set several times.
//
// The values are taken from the option itself rather than from its string
// form: a raw option holds values with commas in them, and reading it back
// through its string form is where a comma would split one value into two.
func (t Input) StringSlice(name string) []string {
	flag := t.flags.Lookup(name)
	if flag == nil {
		return nil
	}
	if v, ok := flag.Value.(pflag.SliceValue); ok {
		return v.GetSlice()
	}
	l, _ := t.flags.GetStringSlice(name)
	return l
}

// Changed reports whether the option was set, which tells an option left at
// its default from one set to the same value on purpose.
func (t Input) Changed(name string) bool {
	flag := t.flags.Lookup(name)
	return flag != nil && flag.Changed
}

// NewCommand returns the command tree of a driver, one command per word of the
// paths its actions name, and one leaf per action.
//
// The tree is complete before anything is parsed, so the options of an action
// are known to the parser, its help is the help of a command the parser knows,
// and a word nobody declared is refused where it is typed rather than reaching
// a driver that cannot make sense of it.
func NewCommand(actions []Action, w io.Writer) (*cobra.Command, error) {
	return NewCommandAs("array", actions, w)
}

// NewCommandAs is NewCommand, showing use as the words that reach these
// actions.
//
// The tree of a driver is reached as "om array <name>", which is three words
// and the name of nothing cobra knows. Use holds one word, because cobra reads
// the name of the command from it and substitutes that name in the usage line;
// the display name annotation is what it builds the path of this command and
// of every command under it from, and it takes all three words.
func NewCommandAs(use string, actions []Action, w io.Writer) (*cobra.Command, error) {
	root := &cobra.Command{
		Use:           "array",
		Annotations:   map[string]string{cobra.CommandDisplayNameAnnotation: use},
		Short:         "manage a storage array",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// The array was named to choose this driver, and is named again here so
	// the parser of this tree accepts it where the user typed it.
	if err := FlagArray.register(root.PersistentFlags()); err != nil {
		return nil, err
	}

	for _, action := range actions {
		if len(action.Path) == 0 {
			return nil, fmt.Errorf("action with no path")
		}
		parent := root
		for _, word := range action.Path[:len(action.Path)-1] {
			parent = group(parent, word)
		}
		leaf, err := newLeaf(action, w)
		if err != nil {
			return nil, err
		}
		parent.AddCommand(leaf)
	}
	return root, nil
}

// RunActions builds the command tree of a driver and runs the arguments
// through it.
//
// This is the whole of what a driver has to do to be driven by a command line,
// and it is also how something that is not a command line drives one: the
// arguments are the ones given here, never those of the process.
func RunActions(ctx context.Context, actions []Action, args []string, w io.Writer) error {
	return RunActionsAs(ctx, "array", actions, args, w)
}

// RunActionsAs is RunActions, showing use as the words that reach these
// actions. A caller reaching a driver through a command line of its own knows
// them; the tree does not.
func RunActionsAs(ctx context.Context, use string, actions []Action, args []string, w io.Writer) error {
	root, err := NewCommandAs(use, actions, w)
	if err != nil {
		return err
	}
	root.SetArgs(args)
	root.SetContext(ctx)
	return root.Execute()
}

// group returns the command of a word on the way to an action, adding it when
// it is the first action to need it.
func group(parent *cobra.Command, word string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == word {
			return cmd
		}
	}
	cmd := &cobra.Command{
		Use:   word,
		Short: word + " commands",

		// A word this one does not lead to is a typo, and saying so beats
		// printing the help of the word that was spelled right. Cobra only
		// checks that for a command it can run, so this one is runnable and
		// shows its help when it is the last word.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	parent.AddCommand(cmd)
	return cmd
}

// newLeaf returns the command running one action.
func newLeaf(action Action, w io.Writer) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:    action.Path[len(action.Path)-1],
		Short:  action.Short,
		Long:   action.Long,
		Hidden: action.Hidden,

		// Every option of an array action is named. A word left over is a
		// mistake.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := action.Run(cmd.Context(), Input{flags: cmd.Flags()})
			if err != nil {
				return err
			}
			if data == nil {
				return nil
			}
			return dump(w, data)
		},
	}
	if err := registerFlags(cmd.Flags(), action.Flags); err != nil {
		return nil, fmt.Errorf("action %s: %w", strings.Join(action.Path, " "), err)
	}
	return cmd, nil
}

// registerFlags declares the options of an action, and the spellings they
// also answer to.
func registerFlags(flags *pflag.FlagSet, l []Flag) error {
	aliases := make(map[string]string)
	for _, flag := range l {
		if err := flag.register(flags); err != nil {
			return err
		}
		for _, alias := range flag.Aliases {
			aliases[alias] = flag.Name
		}
	}
	if len(aliases) == 0 {
		return nil
	}
	flags.SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if canonical, ok := aliases[name]; ok {
			return pflag.NormalizedName(canonical)
		}
		return pflag.NormalizedName(name)
	})
	return nil
}

// dump renders what an action returned.
func dump(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}
