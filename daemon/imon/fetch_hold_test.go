package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
)

// The orchestration holds its actions while the fetch of the resource files
// of a peer, and the ingest closing it, runs, as the provision of a
// non-leader would otherwise wait for the lock the ingest holds and fail on
// its timeout. It holds up to fetchHoldTimeout from the first of the fetches
// running one after the other, and the end of the fetch resumes it.
func TestTheOrchestrationHoldsForTheFetch(t *testing.T) {
	m := newProvisionedTestManager(instance.MonitorStateWaitLeader)
	assert.False(t, m.holdForFetch(), "no fetch running")

	m.files.fetching = true
	m.files.fetchingSince = time.Now()
	assert.True(t, m.holdForFetch(), "a fetch running")
	assert.True(t, m.files.held)

	m.files.fetchingSince = time.Now().Add(-2 * fetchHoldTimeout)
	assert.False(t, m.holdForFetch(), "a fetch running past the bound")
	assert.False(t, m.files.held, "the hold said over")

	m.files.fetchingSince = time.Now()
	require.True(t, m.holdForFetch())
	m.onFetchDone(cmdFetchDone{})
	assert.False(t, m.files.fetching)
	assert.False(t, m.files.held, "the end of the fetch resumes the orchestration")
	assert.True(t, m.files.fetchingSince.IsZero(), "the next fetch begins its own bound")
}
