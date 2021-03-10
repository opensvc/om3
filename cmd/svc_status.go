package cmd

import (
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
)

var svcStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print selected service and instance status",
	Long: `Resources Flags:

(1) R   Running,           . Not Running
(2) M   Monitored,         . Not Monitored
(3) D   Disabled,          . Enabled
(4) O   Optional,          . Not Optional
(5) E   Encap,             . Not Encap
(6) P   Not Provisioned,   . Provisioned
(7) S   Standby,           . Not Standby
(8) <n> Remaining Restart, + if more than 10,   . No Restart

`,
	Run: svcStatusCmdRun,
}

func init() {
	svcCmd.AddCommand(svcStatusCmd)
}

func svcStatusCmdRun(cmd *cobra.Command, args []string) {
	selector := mergeSelector(svcSelectorFlag, "svc", "")
	c := client.NewConfig()
	c.SetURL(serverFlag)
	api, _ := c.NewAPI()
	selection := object.NewSelection(selector)
	selection.SetAPI(api)
	//selection.SetLocal()
	selection.Action("Status")
}
