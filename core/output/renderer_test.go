package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// tabItem is what most commands hand the renderer: a typed value
	// whose json tags name its fields.
	tabItem struct {
		Name      string    `json:"name"`
		Size      int64     `json:"size"`
		Nested    tabNested `json:"nested"`
		Optional  *string   `json:"optional,omitempty"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	tabNested struct {
		Kind string `json:"kind"`
	}

	// tabComposed is a type whose columns are not its fields.
	tabComposed struct {
		Beating bool
	}

	// tabList is the wrapper the api answers with.
	tabList struct {
		Items []tabItem `json:"items"`
	}
)

func (t tabComposed) Unstructured() map[string]any {
	if t.Beating {
		return map[string]any{"state": "beating"}
	}
	return map[string]any{"state": "stale"}
}

func (t tabList) GetItems() any {
	return t.Items
}

func sprintTab(t *testing.T, data any, spec string) string {
	t.Helper()
	s, err := Renderer{
		Output: "tab=" + spec,
		Color:  "no",
		Data:   data,
	}.Sprint()
	require.NoError(t, err)
	return s
}

// TestTabRendersTypedValues is the point of walking the typed data: a
// field is selectable because of the json tag it already carries, and
// nothing has to restate the type for a column to exist.
func TestTabRendersTypedValues(t *testing.T) {
	optional := "here"
	items := []tabItem{
		{Name: "a", Size: 1152921504606846976, Nested: tabNested{Kind: "svc"}, Optional: &optional},
		{Name: "b", Size: 2, Nested: tabNested{Kind: "vol"}},
	}

	t.Run("a column per json tag", func(t *testing.T) {
		// Several columns are aligned, not joined: the comma in a spec
		// separates columns, and only the values a single path finds
		// are joined with one.
		assert.Equal(t, "a  svc  \nb  vol  \n", sprintTab(t, items, "{.name},{.nested.kind}"))
	})

	t.Run("a large int64 keeps its digits", func(t *testing.T) {
		// The json round trip this renderer used to do parsed numbers
		// into float64, and a size came out in exponent form.
		assert.Contains(t, sprintTab(t, items, "{.size}"), "1152921504606846976")
	})

	t.Run("an optional field is its value, not its address", func(t *testing.T) {
		assert.Equal(t, "here\n-\n", sprintTab(t, items, "{.optional}"))
	})

	t.Run("a single item is a single row", func(t *testing.T) {
		assert.Equal(t, "a\n", sprintTab(t, items[0], "{.name}"))
	})

	t.Run("the items of a list wrapper", func(t *testing.T) {
		assert.Equal(t, "a\nb\n", sprintTab(t, tabList{Items: items}, "{.name}"))
	})

	t.Run("a path that resolves to nothing", func(t *testing.T) {
		assert.Equal(t, "-\n-\n", sprintTab(t, items, "{.nosuchfield}"))
	})
}

// TestTabRendersComposedViews covers the types whose columns are not
// their fields: they compose a map, and the renderer reads it.
func TestTabRendersComposedViews(t *testing.T) {
	l := []tabComposed{{Beating: true}, {Beating: false}}
	assert.Equal(t, "beating\nstale\n", sprintTab(t, l, "{.state}"))
}

// TestTabRendersMaps covers the callers that hand over a map because
// they add a column of their own.
func TestTabRendersMaps(t *testing.T) {
	l := []map[string]any{
		{"name": "a", "bin_size": "1gi"},
		{"name": "b", "bin_size": "2gi"},
	}
	assert.Equal(t, "a  1gi  \nb  2gi  \n", sprintTab(t, l, "{.name},{.bin_size}"))
}

// TestTabHeaders pins the alignment the header form asks for.
func TestTabHeaders(t *testing.T) {
	items := []tabItem{{Name: "a"}, {Name: "bbb"}}
	s := sprintTab(t, items, "NAME:.name,KIND:.nested.kind")
	// An empty field is empty. The dash is for a path that resolves to
	// nothing at all.
	assert.Equal(t, "NAME  KIND  \na           \nbbb         \n", s)
}
