package commoncmd

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func parseAudit(t *testing.T, stream string) ([]string, error) {
	t.Helper()
	eventC := make(chan string)
	errC := make(chan error)
	go auditParse(context.Background(), strings.NewReader(stream), eventC, errC)
	// auditParse sends its error before it closes eventC: read both, as
	// the client does.
	var data []string
	for {
		select {
		case msg := <-eventC:
			data = append(data, msg)
		case err := <-errC:
			return data, err
		}
	}
}

func TestAuditParseEnd(t *testing.T) {
	data, err := parseAudit(t, "id:0\ndata:{\"message\":\"started\"}\n\nid:1\ndata:{\"message\":\"log\"}\n\n")
	assert.Equal(t, []string{`{"message":"started"}`, `{"message":"log"}`}, data)
	assert.ErrorIs(t, err, io.EOF, "a stream that ends is reconnected")
}

func TestAuditParsePreempted(t *testing.T) {
	data, err := parseAudit(t, "id:0\ndata:{\"message\":\"started\"}\n\nid:1\nevent:preempted\ndata:{\"message\":\"preempted\"}\n\n")
	assert.Equal(t, []string{`{"message":"started"}`, `{"message":"preempted"}`}, data, "the preempt message is rendered")
	assert.ErrorIs(t, err, errAuditPreempted, "a preempted stream is not reconnected")
}

func TestAuditParseOtherEvent(t *testing.T) {
	_, err := parseAudit(t, "id:0\nevent:other\ndata:{}\n\nid:1\ndata:{}\n\n")
	assert.ErrorIs(t, err, io.EOF)
}
