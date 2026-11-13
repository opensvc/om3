package omcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/daemoncmd"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonRunning struct {
		OptsGlobal
		NodeSelector string
	}
)

// daemonRunningPingTimeout is how long the daemon of a peer is given to
// answer a ping.
const daemonRunningPingTimeout = 5 * time.Second

// Run exits 0 when the daemon of every node selected runs, the local one
// with no selector, and 1 when one does not. A selector naming no cluster
// node is an error.
//
// The daemon of a peer is asked through the api of the local one, which
// passes the ping on: a peer whose daemon does not answer is not running.
func (t *CmdDaemonRunning) Run() error {
	cli, err := client.New()
	if err != nil {
		return err
	}
	nodenames := []string{hostname.Hostname()}
	if t.NodeSelector != "" && t.NodeSelector != hostname.Hostname() {
		nodenames, err = nodeselector.New(t.NodeSelector, nodeselector.WithClient(cli)).Expand()
		if err != nil {
			return err
		}
		if len(nodenames) == 0 {
			return fmt.Errorf("%s: no cluster node selected", t.NodeSelector)
		}
	}
	running := true
	for _, nodename := range nodenames {
		ok, err := t.isRunning(cli, nodename)
		if err != nil {
			return fmt.Errorf("%s: %w", nodename, err)
		}
		running = running && ok
	}
	if !running {
		os.Exit(1)
	}
	return nil
}

func (t *CmdDaemonRunning) isRunning(cli *client.T, nodename string) (bool, error) {
	if nodename == hostname.Hostname() {
		return daemoncmd.New(cli).IsRunning()
	}
	ctx, cancel := context.WithTimeout(context.Background(), daemonRunningPingTimeout)
	defer cancel()
	resp, err := cli.GetNodePing(ctx, nodename)
	if err != nil {
		// The local daemon, which passes the ping on, does not answer:
		// whether the peer runs is not known.
		return false, err
	}
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return false, fmt.Errorf("ping: %s", resp.Status)
	default:
		return false, nil
	}
}
