package actionrouter

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/util/logreader"
)

type (
	// ActionLogs streams the log entries of an action, from the nodes it
	// runs on, as they are logged. It is what --follow shows of an action
	// asked of a daemon, rather than the ids it answered with.
	ActionLogs struct {
		lr      *logreader.LogReader
		readers []event.ReadCloser
		done    chan struct{}
	}
)

const (
	// actionLogsHistory is how many entries of the action logged before
	// the stream was opened are shown: all of them, an action being one
	// command or one orchestration.
	actionLogsHistory = 10000

	// actionLogsTail is how long the stream stays open after the action
	// ended, for its last entries to reach it.
	actionLogsTail = time.Second
)

// StartSessionLogs opens the log streams of a session on nodes, as
// StartActionLogs does.
func StartSessionLogs(c *client.T, nodes []string, sessionID uuid.UUID, format string) (*ActionLogs, error) {
	return StartActionLogs(c, nodes, []string{fmt.Sprintf("SESSION_ID=%s", sessionID)}, format)
}

// StartOrchestrationLogs opens the log streams of orchestrations on every
// node of the cluster, as StartActionLogs does: an orchestration may make
// any node act, and a node it does not reach streams nothing.
//
// The streams open once the orchestrations are accepted, their ids being
// known then only, and show what they logged before from their history.
func StartOrchestrationLogs(c *client.T, ids []uuid.UUID, format string) (*ActionLogs, error) {
	nodes, err := nodeselector.New("*", nodeselector.WithClient(c)).Expand()
	if err != nil {
		return nil, err
	}
	filters := make([]string, len(ids))
	for i, id := range ids {
		// Several values of a field match an entry having any of them.
		filters[i] = fmt.Sprintf("ORCHESTRATION_ID=%s", id)
	}
	return StartActionLogs(c, nodes, filters, format)
}

// StartActionLogs opens the streams of the log entries matching filters on
// nodes, and writes them, sorted by time, to the standard output in format
// until Stop. A node whose stream does not open is reported and skipped.
func StartActionLogs(c *client.T, nodes []string, filters []string, format string) (*ActionLogs, error) {
	return startActionLogs(c, nodes, filters, format, os.Stdout)
}

func startActionLogs(c *client.T, nodes []string, filters []string, format string, w io.Writer) (*ActionLogs, error) {
	t := &ActionLogs{done: make(chan struct{})}
	lines := actionLogsHistory
	follow := true
	streams := make([]logreader.NodeStream, 0, len(nodes))
	for i, node := range nodes {
		reader, err := c.NewGetLogs(node).
			SetFilters(&filters).
			SetLines(&lines).
			SetFollow(&follow).
			GetReader()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s: stream the logs matching %s: %s\n", node, strings.Join(filters, " "), err)
			continue
		}
		t.readers = append(t.readers, reader)
		streams = append(streams, logreader.NodeStream{Node: node, Reader: reader, Index: i})
	}
	if len(streams) == 0 {
		close(t.done)
		return t, fmt.Errorf("no stream of the logs matching %s could be opened", strings.Join(filters, " "))
	}
	t.lr = logreader.New()
	go func() {
		defer close(t.done)
		t.lr.Start(streams, w, logreader.DefaultRenderFunc(format, len(streams)), true)
	}()
	return t, nil
}

// Stop ends the streams, once the last entries of the action had the time
// to reach them, and returns when the entries read are written.
func (t *ActionLogs) Stop() {
	if t == nil || t.lr == nil {
		return
	}
	time.Sleep(actionLogsTail)
	// Close the readers first: the log reader stops a node reader between
	// two reads, and a stream open on a quiet action blocks the read.
	for _, r := range t.readers {
		_ = r.Close()
	}
	t.lr.Stop()
	<-t.done
}
