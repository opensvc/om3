package object

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A collector request timing out says which keyword bounds it.
func TestCollectorRequestErrorNamesTheTimeoutKeyword(t *testing.T) {
	err := collectorRequestError(context.DeadlineExceeded, "POST", "/api/auth/node", 5*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.EqualError(t, err, "collector POST /api/auth/node: no answer within 5s (collector.timeout): context deadline exceeded")

	other := errors.New("connection refused")
	err = collectorRequestError(other, "POST", "/api/auth/node", 5*time.Second)
	assert.EqualError(t, err, "collector POST /api/auth/node: connection refused")
}

// collector.timeout is bounded in the configuration every request reads.
func TestCollectorConfigTimeoutIsBounded(t *testing.T) {
	d := time.Minute
	assert.Equal(t, 20*time.Second, (&CollectorConfigRaw{timeout: &d}).AsConfig().Timeout)
	assert.Equal(t, 5*time.Second, (&CollectorConfigRaw{}).AsConfig().Timeout)
}
