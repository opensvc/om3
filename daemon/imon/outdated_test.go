package imon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOutdatedRefreshDelay(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	require.Equal(t, 46*time.Second, outdatedRefreshDelay(now.Add(45*time.Second), time.Time{}, now), "the margin is added")
	require.Equal(t, 30*time.Second, outdatedRefreshDelay(now.Add(time.Second), now, now), "not before the floor after the last refresh")
	require.Equal(t, time.Duration(0), outdatedRefreshDelay(now.Add(-time.Minute), now.Add(-time.Hour), now), "past due: now")
}
