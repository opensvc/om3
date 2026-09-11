package om

import (
	"fmt"
	"os"
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
)

func init() {
	commoncmd.CmdWithArg(cmdArray, `NAME     The array to act on, named by its section with or without the "array#" prefix.`)
	commoncmd.CmdWithArg(cmdArray, `COMMAND  A command of the driver of that array. "om array NAME" lists them.`)
	root.AddCommand(
		cmdArray,
	)
	cmdArray.AddGroup(
		commoncmd.NewGroupQuery(),
	)
	cmdArray.AddCommand(
		newCmdArrayList(),
	)
}

func runArray(cmd *cobra.Command, args []string) error {
	// The array is named as the first word, which is the form to write, or
	// with the option the command was born with.
	flagName, err := array.NameFromArgs(args)
	if err != nil {
		return err
	}
	argName, rest := array.NameFromFirstArg(args)

	var arrayName string
	switch {
	case argName != "" && flagName != "":
		// Naming it twice can only be a mistake, and picking one of the two
		// would act on an array the operator did not read on the command line.
		return fmt.Errorf("the array is named twice: as an argument and with --%s", array.FlagArray.Name)
	case argName != "":
		arrayName, args = argName, rest
	case flagName != "":
		arrayName = flagName
	default:
		// Nothing names the array whose driver would say what the other words
		// mean, so there is nothing to hand them to.
		return cmd.Help()
	}
	// The name as it was typed is the one a help text shows, so what it
	// prints is a command the reader can type back.
	typedName := arrayName
	if !strings.HasPrefix(arrayName, "array#") {
		arrayName = "array#" + arrayName
	}
	o, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return err
	}
	if !o.MergedConfig().HasSectionString(arrayName) {
		return fmt.Errorf("no section found matching %s in the node or cluster config", arrayName)
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
