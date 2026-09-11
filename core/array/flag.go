package array

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/util/flagvalue"
)

type (
	// Kind is the type of value a flag carries.
	Kind int

	// Flag is one option of an array action.
	//
	// The flags an array driver needs are mostly the same from one array to
	// the next: a name, a size, a serial, a set of mappings. Declaring them
	// here rather than in each driver keeps one spelling and one help text
	// for each, so two arrays never disagree on what --size means, and a
	// driver added later inherits the wording rather than inventing it.
	//
	// A driver needing an option no other array has declares its own Flag
	// value next to the action that uses it.
	Flag struct {
		// Name is the option name, without the leading dashes.
		Name string

		// Shorthand is the single letter form, when the option has one.
		Shorthand string

		// Usage is the one line help of the option.
		Usage string

		// Kind is the type of the value.
		Kind Kind

		// Default is the value of the option when the command line does not
		// set it. It must be of the type Kind names, or nil for the zero
		// value of that type.
		Default any

		// Aliases is the other spellings the option answers to, for the ones
		// a caller out there already writes differently.
		Aliases []string
	}
)

const (
	String Kind = iota
	Int
	Bool

	// StringSlice reads a comma as a separator between values.
	StringSlice

	// RawStringSlice takes each value whole, for an option repeated on the
	// command line whose values hold commas of their own. A mapping is
	// written "<hba>:<tgt>,<tgt>", and splitting it on the comma would turn
	// one mapping into two halves of one.
	RawStringSlice
)

// The options an array driver is likely to need. A driver uses the ones that
// apply to it, and declares its own for what is genuinely its own.
var (
	// FlagArray named the array before the argument did. It is registered on
	// the command tree of every driver so a command line written for the older
	// agent still parses, and its usage points at the form to write.
	FlagArray = Flag{
		Name:      "array",
		Shorthand: "a",
		Usage:     "the array to act on, deprecated by the NAME argument",
		Kind:      String,
	}
	FlagBlocksize = Flag{
		Name:  "blocksize",
		Usage: "disk blocksize in B",
		Kind:  String,
	}
	FlagFilter = Flag{
		Name:  "filter",
		Usage: "items filtering expression",
		Kind:  String,
	}
	FlagForce = Flag{
		Name:  "force",
		Usage: "bypass the sanity checks",
		Kind:  Bool,
	}
	FlagHostGroup = Flag{
		Name:  "hostgroup",
		Usage: "host group name, can be set multiple times",
		Kind:  RawStringSlice,
	}
	FlagID = Flag{
		Name:  "id",
		Usage: "item id",
		Kind:  String,
	}
	FlagLUN = Flag{
		Name:    "lun",
		Usage:   "logical unit number",
		Kind:    Int,
		Default: -1,
	}
	FlagTarget = Flag{
		Name:  "target",
		Usage: "a target name to export the disk through, can be set multiple times",
		Kind:  RawStringSlice,
	}
	// FlagMapping is spelled as v2 spells it, and as the collector writes it.
	FlagMapping = Flag{
		Name:  "mappings",
		Usage: "a <hba_id>:<tgt_id>,<tgt_id>,... mapping, can be set multiple times",
		Kind:  RawStringSlice,
	}
	FlagName = Flag{
		Name:  "name",
		Usage: "item name",
		Kind:  String,
	}
	FlagNAA = Flag{
		Name:  "naa",
		Usage: "the disk naa identifier",
		Kind:  String,
	}
	FlagPool = Flag{
		Name:  "pool",
		Usage: "the pool to create the disk into",
		Kind:  String,
	}
	FlagSerial = Flag{
		Name:  "serial",
		Usage: "item serial",
		Kind:  String,
	}
	FlagSize = Flag{
		Name:  "size",
		Usage: "disk size, expressed as a size expression like 1g, 100mib",
		Kind:  String,
	}
	FlagTruncate = Flag{
		Name:  "truncate",
		Usage: "allow truncating a resized volume (DANGER)",
		Kind:  Bool,
	}
	FlagVolumeGroup = Flag{
		Name:  "volumegroup",
		Usage: "volume group name",
		Kind:  String,
	}
	FlagWWN = Flag{
		Name:  "wwn",
		Usage: "world wide number identifier",
		Kind:  String,
	}
)

// register declares the flag in a flag set.
func (t Flag) register(flags *pflag.FlagSet) error {
	switch t.Kind {
	case String:
		flags.StringP(t.Name, t.Shorthand, t.defaultString(), t.Usage)
	case Int:
		flags.IntP(t.Name, t.Shorthand, t.defaultInt(), t.Usage)
	case Bool:
		flags.BoolP(t.Name, t.Shorthand, t.defaultBool(), t.Usage)
	case StringSlice:
		flags.StringSliceP(t.Name, t.Shorthand, t.defaultStringSlice(), t.Usage)
	case RawStringSlice:
		values := t.defaultStringSlice()
		flagvalue.RawStringSliceVarP(flags, &values, t.Name, t.Shorthand, values, t.Usage)
	default:
		return fmt.Errorf("flag %s: unknown kind %d", t.Name, t.Kind)
	}
	return nil
}

func (t Flag) defaultString() string {
	if s, ok := t.Default.(string); ok {
		return s
	}
	return ""
}

func (t Flag) defaultInt() int {
	if i, ok := t.Default.(int); ok {
		return i
	}
	return 0
}

func (t Flag) defaultBool() bool {
	if b, ok := t.Default.(bool); ok {
		return b
	}
	return false
}

func (t Flag) defaultStringSlice() []string {
	if l, ok := t.Default.([]string); ok {
		return l
	}
	return nil
}

// NameFromFirstArg returns the array named by the first word of a command
// line, and the words that follow it.
//
// "om array <name> <action>" names the array where a reader expects to find
// it, before the words saying what to do with it. A first word beginning with
// a dash is an option and names no array, and neither does no word at all.
func NameFromFirstArg(args []string) (string, []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", args
	}
	return args[0], args[1:]
}

// NameFromArgs returns the array named in a command line, and whether it names
// one.
//
// It reads the arguments it is given rather than those of the process, so an
// array command can be run by something that is not a command line: an api
// handler, a test, or one process driving two arrays.
//
// The parsing is pflag's own, so the array is named the way every other option
// is named: "-a x", "-a=x", "--array x" and "--array=x" all say the same
// thing. The options of the action are not known here and are left alone.
func NameFromArgs(args []string) (string, error) {
	flags := pflag.NewFlagSet("array", pflag.ContinueOnError)
	flags.ParseErrorsWhitelist.UnknownFlags = true
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	var name string
	flags.StringVarP(&name, FlagArray.Name, FlagArray.Shorthand, "", FlagArray.Usage)

	// A line asking for help still names the array whose help it asks for, and
	// the tree of that array is what answers. The parser stops where it meets
	// a help option, so the help options are taken out before it reads: an
	// array named after one of them would otherwise go unseen, and the help
	// of the command that only picks the array would answer instead.
	if err := flags.Parse(withoutHelp(args)); err != nil && !errors.Is(err, pflag.ErrHelp) {
		return "", err
	}
	return name, nil
}

// withoutHelp returns the arguments with the help options removed.
func withoutHelp(args []string) []string {
	l := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
		default:
			l = append(l, arg)
		}
	}
	return l
}
