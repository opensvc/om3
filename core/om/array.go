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
		Use:     "array",
		Short:   "manage storage arrays",
		Long:    `A array is a backend storage provider for pools.`,
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
	arrayName, err := array.NameFromArgs(args)
	if err != nil {
		return err
	}
	if arrayName == "" {
		// Nothing names the array whose driver would say what the other words
		// mean, so there is nothing to hand them to.
		return cmd.Help()
	}
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
		return array.RunActions(cmd.Context(), actioner.Actions(), args, os.Stdout)
	}

	// A driver that has not declared its actions yet still builds and parses
	// a command tree of its own.
	return drv.Run(args)
}
