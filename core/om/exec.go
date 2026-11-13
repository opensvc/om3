package om

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/util/capexec"
)

var (
	xo capexec.T

	execCmd = &cobra.Command{
		Use:   "exec [flags] -- <command> [<arg>...]",
		Short: "execute a command with cappings and limits",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				capexec.NotReady(capexec.ErrNoCommand)
				return capexec.ErrNoCommand
			}
			return nil
		},
		Run: func(_ *cobra.Command, args []string) {
			xo.Exec(args)
		},
	}
)

func init() {
	commoncmd.CmdWithArg(execCmd, "<command> [<arg>...]: the command to execute, with its arguments, after --")
	root.AddCommand(execCmd)
	flags := execCmd.Flags()
	xo.FlagSet(flags)
}
