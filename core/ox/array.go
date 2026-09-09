package ox

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
		Long: `An array is a backend storage provider for pools.

NAME is the array to act on, written as the section holding it with or without
its "array#" prefix. COMMAND and the options after it are the ones the driver
of that array answers to, so "ox array <name>" alone lists them.`,
		Example: `  ox array freenas add disk --name d1 --size 1g
  ox array freenas
  ox array list`,
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
	o, err := object.NewCluster(object.WithVolatile(true))
	if err != nil {
		return err
	}
	if !o.Config().HasSectionString(arrayName) {
		return fmt.Errorf("no section found matching %s in the cluster config", arrayName)
	}
	arrayType, err := o.Config().GetStringStrict(key.T{Section: arrayName, Option: "type"})
	if err != nil {
		return err
	}
	drv := array.GetDriver(arrayType)
	if drv == nil {
		return fmt.Errorf("no array driver found matching type %s", arrayType)
	}
	drv.SetName(arrayName)
	drv.SetConfig(o.Config())

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
