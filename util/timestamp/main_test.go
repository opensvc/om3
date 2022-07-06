package timestamp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimestamp(t *testing.T) {
	ts := Now()
	s := ts.String()
	t.Logf("%s", s)

	require.True(t, NewZero().IsZero())
	require.True(t, New(zero).IsZero())

	var fromString = T{}
	require.NoError(t, json.Unmarshal([]byte("0.0"), &fromString))
	require.True(t, fromString.IsZero())
}
