package commands

import (
	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/entrypoints/action"
	"opensvc.com/opensvc/core/object"
)

type (
	// CmdObjectStatus is the cobra flag set of the status command.
	CmdObjectStatus struct {
		object.OptsStatus
	}
)

// Init configures a cobra command and adds it to the parent command.
func (t *CmdObjectStatus) Init(kind string, parent *cobra.Command, selector *string) {
	cmd := t.cmd(kind, selector)
	parent.AddCommand(cmd)
	object.InstallFlags(cmd, t)
}

func (t *CmdObjectStatus) cmd(kind string, selector *string) *cobra.Command {
	return &cobra.Command{
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
		Run: func(cmd *cobra.Command, args []string) {
			t.run(selector, kind)
		},
	}
}

func (t *CmdObjectStatus) run(selector *string, kind string) {
	a := action.ObjectAction{
		Action: action.Action{
			ObjectSelector: mergeSelector(*selector, t.Global.ObjectSelector, kind, ""),
			NodeSelector:   t.Global.NodeSelector,
			DefaultIsLocal: true,
			Local:          t.Global.Local,
			Format:         t.Global.Format,
			Color:          t.Global.Color,
			Action:         "status",
		},
		Object: object.Action{
			Run: func(path object.Path) (interface{}, error) {
				intf := path.NewObject().(object.Baser)
				return intf.Status(t.OptsStatus)
			},
		},
	}
	action.Do(a)
}
