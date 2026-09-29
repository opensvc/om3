package resource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatusLogChangesAt(t *testing.T) {
	l := NewStatusLog()
	now := time.Now()
	l.ChangesAt(now.Add(-time.Minute))
	require.True(t, l.ChangeAt().IsZero(), "a time past is ignored")
	l.ChangesAt(now.Add(time.Hour))
	l.ChangesAt(now.Add(time.Minute))
	l.ChangesAt(now.Add(2 * time.Minute))
	require.Equal(t, now.Add(time.Minute), l.ChangeAt(), "the earliest is kept")
	l.Reset()
	require.True(t, l.ChangeAt().IsZero())
}

func TestStatusLogRPOBreachesAt(t *testing.T) {
	l := NewStatusLog()
	past := time.Now().Add(-time.Hour)
	l.RPOBreachesAt(time.Now().Add(time.Hour))
	l.RPOBreachesAt(past)
	require.Equal(t, past, l.RPOBreachAt(), "a breach in the past is kept: it is breached")
	l.Reset()
	require.True(t, l.RPOBreachAt().IsZero())
}
