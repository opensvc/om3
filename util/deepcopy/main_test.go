package deepcopy

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnyLeavesNoSharedReference(t *testing.T) {
	original := map[string]any{
		"scalar": 1,
		"slice":  []any{"a", map[string]any{"nested": "value"}},
		"map":    map[string]any{"inner": []any{1, 2}},
		"typed":  []string{"x", "y"},
	}
	copied := Any(original)
	require.Equal(t, original, copied)

	// Reach through every level of the original and check none of it
	// wrote through to the copy.
	original["scalar"] = 2
	original["slice"].([]any)[0] = "mutated"
	original["slice"].([]any)[1].(map[string]any)["nested"] = "mutated"
	original["map"].(map[string]any)["inner"].([]any)[0] = 99
	original["typed"].([]string)[0] = "mutated"

	assert.Equal(t, 1, copied["scalar"])
	assert.Equal(t, "a", copied["slice"].([]any)[0])
	assert.Equal(t, "value", copied["slice"].([]any)[1].(map[string]any)["nested"])
	assert.Equal(t, 1, copied["map"].(map[string]any)["inner"].([]any)[0])
	assert.Equal(t, "x", copied["typed"].([]string)[0])
}

func TestAnyKeepsNilApartFromEmpty(t *testing.T) {
	// A nil and an empty collection do not serialize alike, and the
	// datasets have fields carrying no omitempty, so the difference has
	// to survive the copy.
	var nilMap map[string]any
	require.Nil(t, Any(nilMap))
	require.NotNil(t, Any(map[string]any{}))

	var nilSlice []string
	require.Nil(t, Any(nilSlice))
	require.NotNil(t, Any([]string{}))

	require.Nil(t, Any[any](nil))
}

func TestAnyCopiesPointersAndStructs(t *testing.T) {
	type inner struct{ Items []int }
	type outer struct {
		Name  string
		Inner *inner
		When  time.Time
	}
	now := time.Now()
	original := &outer{Name: "a", Inner: &inner{Items: []int{1, 2}}, When: now}
	copied := Any(original)

	require.NotSame(t, original, copied)
	require.NotSame(t, original.Inner, copied.Inner)
	assert.True(t, copied.When.Equal(now), "an unexported-only struct is copied whole")

	original.Inner.Items[0] = 99
	original.Name = "b"
	assert.Equal(t, []int{1, 2}, copied.Inner.Items)
	assert.Equal(t, "a", copied.Name)
}

func TestAnyMatchesTheJSONRoundTripItReplaces(t *testing.T) {
	original := map[string]any{
		"a": []any{1.0, "two", map[string]any{"three": 3.0}},
		"b": nil,
		"c": map[string]any{},
	}
	viaCopy, err := json.Marshal(Any(original))
	require.NoError(t, err)
	b, err := json.Marshal(original)
	require.NoError(t, err)
	var roundTripped map[string]any
	require.NoError(t, json.Unmarshal(b, &roundTripped))
	viaJSON, err := json.Marshal(roundTripped)
	require.NoError(t, err)
	assert.Equal(t, string(viaJSON), string(viaCopy))
}

func TestSlice(t *testing.T) {
	type named []string

	var nilSlice named
	assert.Nil(t, Slice(nilSlice))

	empty := named{}
	assert.NotNil(t, Slice(empty))
	assert.Len(t, Slice(empty), 0)

	original := named{"a", "b"}
	copied := Slice(original)
	assert.Equal(t, original, copied)
	original[0] = "mutated"
	assert.Equal(t, named{"a", "b"}, copied, "the backing array must not be shared")
}
