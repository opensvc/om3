package plog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// auditStarts returns the number of "start auditing" messages queued in q.
func auditStarts(q chan LogMessage) int {
	n := 0
	for {
		select {
		case m := <-q:
			if m.Message == "start auditing" {
				n++
			}
		default:
			return n
		}
	}
}

// A subsystem attaching the same audit session twice, from the audit
// registry at its start and then from the AuditStart event, says start
// auditing once. Another session is attached, and says it.
func TestHandleAuditStartOncePerSession(t *testing.T) {
	log := NewDefaultLogger()
	q := make(chan LogMessage, 10)

	log.HandleAuditStart(q, nil, "collector")
	log.HandleAuditStart(q, nil, "collector")
	assert.Equal(t, 1, auditStarts(q))
	assert.Equal(t, q, log.Q())

	other := make(chan LogMessage, 10)
	log.HandleAuditStart(other, nil, "collector")
	assert.Equal(t, 1, auditStarts(other))
	assert.Equal(t, other, log.Q())
}
