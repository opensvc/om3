package commoncmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdDaemonSubAction struct {
		Debug        bool
		NodeSelector string

		// OnResponse, when set, receives the body each node answered
		// with. It is called from the goroutine handling that node, so
		// an implementation shared by several nodes serializes itself.
		OnResponse func(nodename string, b []byte)
	}

	apiFuncWithNode func(context.Context, *client.T, string) (*http.Response, error)
)

// maxSubActionBodySize bounds the daemon answer read into memory. The
// answers are a problem document or a small json object.
const maxSubActionBodySize = 1 << 20

// RunForEach runs the action once per name, so a command taking several
// names attempts them all rather than stopping on the first that fails,
// and reports every failure.
func (t *CmdDaemonSubAction) RunForEach(names []string, fn func(name string) apiFuncWithNode) error {
	var errs error
	for _, name := range names {
		errs = errors.Join(errs, t.Run(fn(name)))
	}
	return errs
}

// Run daemon sub-component action
func (t *CmdDaemonSubAction) Run(fn apiFuncWithNode) error {
	if t.NodeSelector == "" {
		if clientcontext.IsSet() {
			return fmt.Errorf("--node must be set")
		}
		t.NodeSelector = hostname.Hostname()
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if errors.Is(err, nodeselector.ErrClusterNodeCacheEmpty) {
		nodenames = []string{hostname.Hostname()}
	} else if err != nil {
		return err
	}
	errC := make(chan error)
	ctx := context.Background()
	running := 0
	localLast := false
	for _, nodename := range nodenames {
		if nodename == hostname.Hostname() {
			localLast = true
			continue
		}
		running++
		go func(nodename string) {
			errC <- t.doNode(ctx, c, nodename, fn)
		}(nodename)
	}
	var (
		errs error
	)
	for {
		if running == 0 {
			break
		}
		err := <-errC
		errs = errors.Join(errs, err)
		running--
	}
	if localLast {
		err := t.doNode(ctx, c, hostname.Hostname(), fn)
		errs = errors.Join(errs, err)
	}
	return errs
}

func (t *CmdDaemonSubAction) doNode(ctx context.Context, cli *client.T, nodename string, fn apiFuncWithNode) error {
	resp, err := fn(ctx, cli, nodename)
	if err != nil {
		return fmt.Errorf("action failed on node %s: %w", nodename, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A caller that read the body itself, as the generated clients with
	// a typed response do, hands over a body that is closed. There is
	// nothing left to say about the answer then, which is not a reason
	// to fail the action.
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxSubActionBodySize))
	if err != nil {
		b = nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The daemon says why in the body. Reporting only the status
		// code makes the caller guess what a 400 was about.
		if s := problemString(b); s != "" {
			return fmt.Errorf("action failed on node %s: %s", nodename, s)
		}
		return fmt.Errorf("action failed on node %s: unexpected status code %d", nodename, resp.StatusCode)
	}
	if t.OnResponse != nil {
		t.OnResponse(nodename, b)
	}
	return nil
}

// problemString renders the problem document a daemon answers an error
// with, or an empty string when the body is not one.
func problemString(b []byte) string {
	var problem api.Problem
	if err := json.Unmarshal(b, &problem); err != nil {
		return ""
	}
	switch {
	case problem.Title != "" && problem.Detail != "":
		return problem.Title + ": " + problem.Detail
	case problem.Title != "":
		return problem.Title
	default:
		return problem.Detail
	}
}
