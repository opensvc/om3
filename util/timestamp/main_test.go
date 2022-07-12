package timestamp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestJSON(t *testing.T) {
	var ts T
	b := []byte("1000.000000001")
	err := json.Unmarshal(b, &ts)
	assert.NoError(t, err)
	assert.Equal(t, ts.tm.Nanosecond(), 1)
	b2, err := json.Marshal(ts)
	assert.Equal(t, string(b), string(b2))
}
