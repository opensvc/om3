package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An item answered by a daemon of an earlier version has no evaluated text:
// the value is shown as it came, a number as the number, not as the float it
// was decoded into.
func TestWithEvaluatedText(t *testing.T) {
	var items KeywordItems
	require.NoError(t, json.Unmarshal([]byte(`[
		{"keyword": "a", "evaluated": 10000000000},
		{"keyword": "b", "evaluated": "hello"},
		{"keyword": "c", "evaluated": ["x", "y"]},
		{"keyword": "d", "evaluated": 10000000000, "evaluated_text": "10s"},
		{"keyword": "e"}
	]`), &items))
	items = WithEvaluatedText(items)
	text := func(i int) string {
		if items[i].EvaluatedText == nil {
			return "<nil>"
		}
		return *items[i].EvaluatedText
	}
	assert.Equal(t, "10000000000", text(0))
	assert.Equal(t, "hello", text(1))
	assert.Equal(t, `["x","y"]`, text(2))
	assert.Equal(t, "10s", text(3), "the text a daemon answered is kept")
	assert.Equal(t, "<nil>", text(4), "no evaluated value, no text")
}
