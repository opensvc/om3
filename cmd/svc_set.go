package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	svcSetNodeFlag  string
	svcSetLocalFlag bool
	svcSetKWFlag    []string
)

var svcSetCmd = &cobra.Command{
	Use:   "get",
	Short: "Set configuration keys",
	Run:   svcSetCmdRun,
}

func init() {
	svcCmd.AddCommand(svcSetCmd)
	svcSetCmd.Flags().BoolVarP(&svcSetLocalFlag, "local", "", false, "Set from the local instance")
	svcSetCmd.Flags().StringVar(&svcSetNodeFlag, "node", "", "Set from the specified nodes")
	svcSetCmd.Flags().StringSliceVar(&svcSetKWFlag, "kw", []string{}, "A keyword to get")
}

func svcSetCmdRun(cmd *cobra.Command, args []string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcSetNodeFlag,
		Local:          svcSetLocalFlag,
		DefaultIsLocal: true,
		Action:         "set",
		Method:         "Set",
		MethodArgs: []interface{}{
			svcSetKWFlag,
		},
		Format: formatFlag,
		Color:  colorFlag,
	}
	action.Do(a)
}
