package om

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	cmdArray = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "array [NAME] [COMMAND]",
		Short:   "manage storage arrays",
		Long:    `An array is a backend storage provider for pools.`,
		Example: `  om array freenas add disk --name d1 --size 1g
  om array freenas
  om array list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArray(cmd, args)
		},

		// The words and options after "array" are the ones of the driver of
		// the array they name, and which driver that is is only known once
		// the array is. They are left untouched here and parsed once, by the
		// command tree of that driver.
		//
		// A global option therefore goes before the "array" word, where the
		// root command parses it.
		DisableFlagParsing: true,
	}

	// cmdNodeArray is the "om node array" command of v2, which the
	// collector still queues its array actions as: a disk the collector
	// form allocates reaches the array proxy node as "om node array add
	// disk -a <array> ...". It is kept for them, and not offered.
	cmdNodeArray = &cobra.Command{
		Use:    "array",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArray(cmd, args)
		},
		DisableFlagParsing: true,
	}
)

func init() {
	commoncmd.CmdWithArg(cmdArray, `NAME     The array to act on, named by its section with or without the "array#" prefix.`)
	commoncmd.CmdWithArg(cmdArray, `COMMAND  A command of the driver of that array. "om array NAME" lists them.`)
	root.AddCommand(
		cmdArray,
	)
	cmdNode.AddCommand(
		cmdNodeArray,
	)
	cmdArray.AddGroup(
		commoncmd.NewGroupQuery(),
	)
	cmdArray.AddCommand(
		newCmdArrayList(),
	)
}

// arrayNameFromArgs returns the array a command line names, and the words of
// the action of its driver, empty when nothing names an array.
//
// The array is named as the first word, which is the form to write, or with
// the option the command was born with. Named with the option, the first word
// is the first word of the action, as in the v2 form "array add disk -a
// <array>" the collector queues its actions as, unless it names an array too,
// which is a mistake rather than a precedence question: picking one of the
// two would act on an array the operator did not read on the command line.
// The option stays in the words handed to the driver, whose command tree
// takes it.
func arrayNameFromArgs(args []string, isArray func(string) bool) (string, []string, error) {
	flagName, err := array.NameFromArgs(args)
	if err != nil {
		return "", nil, err
	}
	argName, rest := array.NameFromFirstArg(args)
	switch {
	case argName != "" && flagName != "" && isArray(argName):
		return "", nil, fmt.Errorf("the array is named twice: as an argument and with --%s", array.FlagArray.Name)
	case flagName != "":
		return flagName, args, nil
	case argName != "":
		return argName, rest, nil
	default:
		return "", args, nil
	}
}

func runArray(cmd *cobra.Command, args []string) error {
	o, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return err
	}
	arrayName, args, err := arrayNameFromArgs(args, func(name string) bool { return hasArraySection(o, name) })
	if err != nil {
		return err
	}
	if arrayName == "" {
		// Nothing names the array whose driver would say what the other words
		// mean, so there is nothing to hand them to.
		return cmd.Help()
	}
	// The name as it was typed is the one a help text shows, so what it
	// prints is a command the reader can type back.
	typedName := arrayName
	arrayName, err = arraySection(o.MergedConfig().SectionStrings(), arrayName, func(section string) string {
		return o.MergedConfig().GetString(key.T{Section: section, Option: "name"})
	})
	if err != nil {
		return err
	}
	arrayType, err := o.MergedConfig().GetStringStrict(key.T{Section: arrayName, Option: "type"})
	if err != nil {
		return err
	}
	drv := array.GetDriver(arrayType)
	if drv == nil {
		return fmt.Errorf("no array driver found matching type %s", arrayType)
	}
	drv.SetName(arrayName)
	drv.SetConfig(o.MergedConfig())

	if actioner, ok := drv.(array.Actioner); ok {
		// The words that reached here are the words a help text has to show,
		// so it names the array the actions are of.
		use := cmd.CommandPath() + " " + typedName
		return array.RunActionsAs(cmd.Context(), use, actioner.Actions(), args, os.Stdout)
	}

	// A driver that has not declared its actions yet still builds and parses
	// a command tree of its own.
	return drv.Run(args)
}

// arraySection returns the array section name designates: the section
// array#<name>, or else the array section whose name keyword is name, as v2
// resolved it. A symmetrix array is named after its serial number this way,
// the serial the remote half of an SRDF pair is reported with, and the
// collector names the array to act on the remote half with.
//
// Two sections having that name is an error rather than a pick of the first,
// which would act on an array the operator did not mean.
func arraySection(sections []string, name string, nameOf func(section string) string) (string, error) {
	ref := name
	if !strings.HasPrefix(ref, "array#") {
		ref = "array#" + ref
	}
	if slices.Contains(sections, ref) {
		return ref, nil
	}
	matches := make([]string, 0)
	for _, section := range sections {
		if !strings.HasPrefix(section, "array#") {
			continue
		}
		if nameOf(section) == name {
			matches = append(matches, section)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no section found matching %s, nor array section with name=%s, in the node or cluster config", ref, name)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("array %s is ambiguous: the sections %s all have name=%s", name, strings.Join(matches, ", "), name)
	}
}

// hasArraySection is true when the node or cluster config has an array
// section of that name, given with or without the "array#" prefix.
func hasArraySection(o *object.Node, name string) bool {
	if !strings.HasPrefix(name, "array#") {
		name = "array#" + name
	}
	return o.MergedConfig().HasSectionString(name)
}
