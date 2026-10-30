package object

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

// The listener throttle keywords are read under their former rate_limiter
// names too, scoped or not, so a cluster configuration written before the
// rename keeps its throttle.
func TestListenerThrottleReadsItsFormerNames(t *testing.T) {
	o, err := newCcfg(naming.Cluster, WithConfigData([]byte(`
[listener]
rate_limiter_rate = 30
rate_limiter_burst@`+hostname.Hostname()+` = 200
rate_limiter_expires = 2m
`)), WithVolatile(true))
	require.NoError(t, err)
	cfg := o.Config()
	assert.Equal(t, 30, cfg.GetInt(key.New("listener", "throttle_rate")))
	assert.Equal(t, 200, cfg.GetInt(key.New("listener", "throttle_burst")))
	expires := cfg.GetDuration(key.New("listener", "throttle_expires"))
	require.NotNil(t, expires)
	assert.Equal(t, 2*time.Minute, *expires)
}
