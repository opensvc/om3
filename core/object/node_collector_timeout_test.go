package object

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/httphelper"
)

// A collector request timing out says which keyword bounds it.
func TestCollectorRequestErrorNamesTheTimeoutKeyword(t *testing.T) {
	err := collectorRequestError(context.DeadlineExceeded, "POST", "/api/auth/node", 5*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.EqualError(t, err, "collector POST /api/auth/node: timed out after 5s (collector.timeout): context deadline exceeded")

	other := errors.New("connection refused")
	err = collectorRequestError(other, "POST", "/api/auth/node", 5*time.Second)
	assert.EqualError(t, err, "collector POST /api/auth/node: connection refused")
}

// collector.timeout is bounded in the configuration every request reads.
func TestCollectorConfigTimeoutIsBounded(t *testing.T) {
	d := time.Minute
	assert.Equal(t, 20*time.Second, (&CollectorConfigRaw{timeout: &d}).AsConfig().Timeout)
	assert.Equal(t, 10*time.Second, (&CollectorConfigRaw{}).AsConfig().Timeout)
}

// A collector request timing out says the phase it was in, the dns
// resolution timing out as the collector.timeout ending it.
func TestCollectorRequestErrorSaysThePhase(t *testing.T) {
	err := collectorRequestError(&httphelper.PhaseError{Phase: httphelper.PhaseResolve, Err: context.DeadlineExceeded}, "POST", "/api/auth/node", 10*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.EqualError(t, err, "collector POST /api/auth/node: timed out after 10s (collector.timeout) while resolving the host name: context deadline exceeded")

	dnsErr := &net.DNSError{Err: "i/o timeout", Name: "collector.example.com", IsTimeout: true}
	err = collectorRequestError(&httphelper.PhaseError{Phase: httphelper.PhaseResolve, Err: dnsErr}, "POST", "/api/auth/node", 10*time.Second)
	assert.EqualError(t, err, "collector POST /api/auth/node: timed out while resolving the host name: lookup collector.example.com: i/o timeout")

	err = collectorRequestError(&httphelper.PhaseError{Phase: httphelper.PhaseConnect, Err: errors.New("connection refused")}, "POST", "/api/auth/node", 10*time.Second)
	assert.EqualError(t, err, "collector POST /api/auth/node: connection refused")
}
