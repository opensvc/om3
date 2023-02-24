package callcount_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/util/callcount"
)

func TestCounter(t *testing.T) {
	mapping := map[int]string{
		1: "operation 1",
		2: "operation 2",
		3: "operation 3",
	}
	c, cancel := callcount.Start(context.Background(), mapping, time.Millisecond)
	defer cancel()
	for _, i := range []int{1, 1, 2, 8, 9, 5, 9} {
		c <- i
	}
	expectedCounts := callcount.Counts{
		1: 2,
		2: 1,
		8: 1,
		5: 1,
		9: 2,
	}
	expectedStats := callcount.Stats{
		"operation 1": 2,
		"operation 2": 1,
		"unknown":     4,
	}

	zeroCounts := callcount.Counts{}
	require.Equal(t, expectedCounts, callcount.Get(c))
	require.Equal(t, expectedStats, callcount.GetStats(c))
	callcount.Reset(c)
	require.Equal(t, zeroCounts, callcount.Get(c))
	require.Equal(t, zeroCounts, callcount.Get(c))

	c <- 9
	require.Equal(t, callcount.Counts{9: 1}, callcount.Get(c))

	callcount.Reset(c)
	require.Equal(t, zeroCounts, callcount.Get(c))
}
