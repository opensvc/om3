package commoncmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonLog struct {
		CmdDaemonSubAction
		Level string
	}
)

func NewCmdDaemonLog() *cobra.Command {
	options := CmdDaemonLog{}
	cmd := &cobra.Command{
		Use:   "log",
		Short: "report or set the level the daemon logs are emitted at",
		Long: "Report the level the daemon logs are emitted at, or set it with --level.\n\n" +
			"The daemon log is written to journald, which is not given anything below\n" +
			"the info level, so info is the most verbose level this accepts. Use\n" +
			"'om daemon audit' for a debug or trace feed: it is read from the daemon\n" +
			"as it runs and is not stored.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagDaemonLogLevel(flags, &options.Level)

	return cmd
}

func (t *CmdDaemonLog) Run() error {
	var (
		mu     sync.Mutex
		levels = make(map[string]string)
	)

	t.OnResponse = func(nodename string, b []byte) {
		var payload api.LogControl
		if err := json.Unmarshal(b, &payload); err != nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		levels[nodename] = payload.Level
	}

	fn := func(ctx context.Context, c *client.T, nodename string) (*http.Response, error) {
		if t.Level == "" {
			return c.GetDaemonLogControl(ctx, nodename)
		}
		return c.PostDaemonLogControl(ctx, nodename, api.PostDaemonLogControlJSONRequestBody{Level: t.Level})
	}

	err := t.CmdDaemonSubAction.Run(fn)

	// Print what was learned even when a node failed: the levels of the
	// nodes that answered are still the answer to the question asked.
	nodenames := make([]string, 0, len(levels))
	for nodename := range levels {
		nodenames = append(nodenames, nodename)
	}
	sort.Strings(nodenames)
	for _, nodename := range nodenames {
		fmt.Printf("%s: %s\n", nodename, levels[nodename])
	}
	return err
}
