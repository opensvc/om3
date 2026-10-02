package actionrouter

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/util/logreader"
)

type (
	// SessionLogs streams the log entries of one session, from the nodes
	// it runs on, as they are logged. It is what --watch shows of an action
	// asked of a daemon, rather than the ids of the execs it started.
	SessionLogs struct {
		lr      *logreader.LogReader
		readers []event.ReadCloser
		done    chan struct{}
	}
)

const (
	// sessionLogsHistory is how many entries of the session logged before
	// the stream was opened are shown: all of them, a session being one
	// command.
	sessionLogsHistory = 10000

	// sessionLogsTail is how long the stream stays open after the execs of
	// the session ended, for their last entries to reach it.
	sessionLogsTail = time.Second
)

// StartSessionLogs opens the log streams of a session on nodes, and writes
// their entries, sorted by time, to the standard output in format until
// Stop. A node whose stream does not open is reported and skipped.
func StartSessionLogs(c *client.T, nodes []string, sessionID uuid.UUID, format string) (*SessionLogs, error) {
	return startSessionLogs(c, nodes, sessionID, format, os.Stdout)
}

func startSessionLogs(c *client.T, nodes []string, sessionID uuid.UUID, format string, w io.Writer) (*SessionLogs, error) {
	t := &SessionLogs{done: make(chan struct{})}
	filters := []string{fmt.Sprintf("SESSION_ID=%s", sessionID)}
	lines := sessionLogsHistory
	follow := true
	streams := make([]logreader.NodeStream, 0, len(nodes))
	for i, node := range nodes {
		reader, err := c.NewGetLogs(node).
			SetFilters(&filters).
			SetLines(&lines).
			SetFollow(&follow).
			GetReader()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%s: stream the logs of session %s: %s\n", node, sessionID, err)
			continue
		}
		t.readers = append(t.readers, reader)
		streams = append(streams, logreader.NodeStream{Node: node, Reader: reader, Index: i})
	}
	if len(streams) == 0 {
		close(t.done)
		return t, fmt.Errorf("no log stream of session %s could be opened", sessionID)
	}
	t.lr = logreader.New()
	go func() {
		defer close(t.done)
		t.lr.Start(streams, w, logreader.DefaultRenderFunc(format, len(streams)), true)
	}()
	return t, nil
}

// Stop ends the streams, once the last entries of the session had the time
// to reach them, and returns when the entries read are written.
func (t *SessionLogs) Stop() {
	if t == nil || t.lr == nil {
		return
	}
	time.Sleep(sessionLogsTail)
	// Close the readers first: the log reader stops a node reader between
	// two reads, and a stream open on a quiet session blocks the read.
	for _, r := range t.readers {
		_ = r.Close()
	}
	t.lr.Stop()
	<-t.done
}
