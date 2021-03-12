package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type (
	flagSetGlobal struct {
		Color  string
		Format string
		Server string
	}

	flagSetObject struct {
		ObjectSelector string
	}

	flagSetAsync struct {
		Watch       bool
		Wait        bool
		WaitTimeout time.Duration
	}

	flagSetAction struct {
		Local        bool
		NodeSelector string
	}
)

func (t *flagSetGlobal) init(cmd *cobra.Command) {
	cmd.Flags().StringVar(&t.Color, "color", "auto", "output colorization yes|no|auto")
	cmd.Flags().StringVar(&t.Format, "format", "auto", "output format json|flat|auto")
	cmd.Flags().StringVar(&t.Server, "server", "", "uri of the opensvc api server. scheme raw|https")
}

func (t *flagSetObject) init(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&t.ObjectSelector, "selector", "s", "", "The name of the object to select")
}

func (t *flagSetAsync) init(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&t.Watch, "watch", "w", false, "Watch the monitor changes")
	cmd.Flags().BoolVar(&t.Wait, "wait", false, "Wait for the object to reach the target state")
	cmd.Flags().DurationVar(&t.WaitTimeout, "time", 5*time.Minute, "Stop waiting for the object to reach the target state after a duration")
}

func (t *flagSetAction) init(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&t.Local, "local", false, "Inline action on local instance")
	cmd.Flags().StringVar(&t.NodeSelector, "node", "", "Execute on a list of nodes")
}

func mergeSelector(selector string, subsysSelector string, kind string, defaultSelector string) string {
	var s string
	switch {
	case selector != "":
		s = selector
	case subsysSelector != "" && kind != "":
		s = fmt.Sprintf("%s+*/%s/*", subsysSelector, kind)
	case subsysSelector != "" && kind == "":
		s = subsysSelector
	case kind != "":
		s = fmt.Sprintf("%s+*/%s/*", defaultSelector, kind)
	default:
		s = defaultSelector
	}
	return s
}
