package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/array"
	"opensvc.com/opensvc/core/commands"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/key"
)

var (
	arrayName string
	arrayCmd  = &cobra.Command{
		Use:   "array",
		Short: "Manage storage arrays",
		Long:  ` A array is backend storage provider for pools.`,
		Run: func(_ *cobra.Command, args []string) {
			if err := runArray(args); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}
)

func init() {
	var (
		cmdArrayLs commands.ArrayLs
	)
	root.AddCommand(arrayCmd)
	arrayCmd.PersistentFlags().StringVar(&arrayName, "array", "", "the section name or index identifying the array")

	cmdArrayLs.Init(arrayCmd)
	//arrayfreenas.InitCommands(arrayCmd)
}

func runArray(args []string) error {
	o, err := object.NewCcfg("cluster", object.WithVolatile(true))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(arrayName, "array#") {
		arrayName = "array#" + arrayName
	}
	if !o.Config().HasSectionString(arrayName) {
		return errors.Errorf("no section found matching %s in the cluster config", arrayName)
	}
	arrayType, err := o.Config().GetStringStrict(key.T{arrayName, "type"})
	if err != nil {
		return err
	}
	drv := array.GetDriver(arrayType)
	if drv == nil {
		return errors.Errorf("no array driver found matching type %s", arrayType)
	}
	drv.SetName(arrayName)
	drv.SetConfig(o.Config())
	return drv.Run(args)
}
