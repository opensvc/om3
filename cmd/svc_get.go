package cmd

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	svcGetNodeFlag  string
	svcGetLocalFlag bool
	svcGetKWFlag    []string
)

var svcGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a configuration key raw value",
	Run:   svcGetCmdRun,
}

func init() {
	svcCmd.AddCommand(svcGetCmd)
	svcGetCmd.Flags().BoolVarP(&svcGetLocalFlag, "local", "", false, "Get from the local instance")
	svcGetCmd.Flags().StringVar(&svcGetNodeFlag, "node", "", "Get from the specified nodes")
	svcGetCmd.Flags().StringSliceVar(&svcGetKWFlag, "kw", []string{}, "A keyword to get")
}

func svcGetCmdRun(cmd *cobra.Command, args []string) {
	a := action.ObjectAction{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcGetNodeFlag,
		Local:          svcGetLocalFlag,
		DefaultIsLocal: true,
		Action:         "get",
		Method:         "Get",
		MethodArgs: []interface{}{
			svcGetKWFlag,
		},
		Format: formatFlag,
		Color:  colorFlag,
	}
	action.Do(a)
}
