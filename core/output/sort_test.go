package output

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sortItem struct {
	Name      string     `json:"name"`
	Rank      int        `json:"rank"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

func names(items []sortItem) []string {
	l := make([]string, len(items))
	for i, item := range items {
		l[i] = item.Name
	}
	return l
}

func TestSortDataOrdersOnSeveralFields(t *testing.T) {
	items := []sortItem{
		{Name: "c", Rank: 1},
		{Name: "a", Rank: 2},
		{Name: "b", Rank: 1},
	}
	require.NoError(t, sortData(items, "rank,name"))
	assert.Equal(t, []string{"b", "c", "a"}, names(items))
}

func TestSortDataReversesATermPrefixedWithAMinus(t *testing.T) {
	items := []sortItem{
		{Name: "b", Rank: 1},
		{Name: "a", Rank: 2},
	}
	require.NoError(t, sortData(items, "-rank"))
	assert.Equal(t, []string{"a", "b"}, names(items))
}

// An instant is ordered as an instant. Two nodes of one cluster can report the
// same moment with different utc offsets, and the text of those does not order
// the way the moments do: "2026-01-01T00:30:00+02:00" sorts after
// "2026-01-01T00:00:00-05:00" as text, and before it as a moment.
func TestSortDataOrdersInstantsAndNotTheirText(t *testing.T) {
	east := time.FixedZone("east", 2*3600)
	west := time.FixedZone("west", -5*3600)
	items := []sortItem{
		{Name: "later", StartedAt: time.Date(2026, 1, 1, 0, 30, 0, 0, east)},
		{Name: "earlier", StartedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, west)},
	}
	require.NoError(t, sortData(items, "started_at"))
	assert.Equal(t, []string{"later", "earlier"}, names(items),
		"00:30+02:00 is 22:30 UTC the day before, so it comes first")
}

// A listing puts what it knows first, and reversing the order is not a reason
// to lead with what it does not.
func TestSortDataPutsAbsentValuesLastEitherWay(t *testing.T) {
	ended := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	items := []sortItem{
		{Name: "running"},
		{Name: "ended", EndedAt: &ended},
	}
	require.NoError(t, sortData(items, "ended_at"))
	assert.Equal(t, []string{"ended", "running"}, names(items))
	require.NoError(t, sortData(items, "-ended_at"))
	assert.Equal(t, []string{"ended", "running"}, names(items))
}

// A sort is stable, so the terms that follow decide and the input order
// decides what they leave equal.
func TestSortDataIsStable(t *testing.T) {
	items := []sortItem{
		{Name: "first", Rank: 1},
		{Name: "second", Rank: 1},
		{Name: "third", Rank: 1},
	}
	require.NoError(t, sortData(items, "rank"))
	assert.Equal(t, []string{"first", "second", "third"}, names(items))
}

func TestSortDataRejectsAnUnparsableTerm(t *testing.T) {
	items := []sortItem{{Name: "a"}}
	assert.Error(t, sortData(items, "{.a"))
}

// A misspelled field that quietly does nothing is worse than one that says so.
func TestSortDataRejectsAFieldNoItemHas(t *testing.T) {
	items := []sortItem{{Name: "b"}, {Name: "a"}}
	err := sortData(items, "nmae")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no such field")
	assert.Equal(t, []string{"b", "a"}, names(items), "and orders nothing")
}

// A field every item has and none has anything in is absent, not unknown: an
// exec that has not ended has an ended_at, and it is empty.
func TestSortDataAcceptsAFieldEveryItemLeavesEmpty(t *testing.T) {
	items := []sortItem{{Name: "b"}, {Name: "a"}}
	assert.NoError(t, sortData(items, "ended_at"))
}

// "." names the item itself, for a listing whose items are bare values.
func TestSortDataOrdersBareValuesOnThemselves(t *testing.T) {
	items := []string{"c", "a", "b"}
	require.NoError(t, sortData(items, "."))
	assert.Equal(t, []string{"a", "b", "c"}, items)
	require.NoError(t, sortData(items, "-."))
	assert.Equal(t, []string{"c", "b", "a"}, items)
}

func TestSortDataLeavesWhatItCannotOrder(t *testing.T) {
	items := []sortItem{{Name: "b"}, {Name: "a"}}
	require.NoError(t, sortData(items, ""))
	assert.Equal(t, []string{"b", "a"}, names(items), "no expression, no reordering")
	require.NoError(t, sortData(nil, "name"))
	require.NoError(t, sortData(sortItem{Name: "a"}, "name"), "one item is already in order")
}

// The default is what the listing comes in, "+" extends it, and anything else
// replaces it.
func TestRendererSortOverridesTheDefault(t *testing.T) {
	for _, tc := range []struct {
		defaultSort string
		sort        string
		expected    []string
	}{
		{"rank", "", []string{"c", "b", "a"}}, // stable: rank 1 in input order
		{"rank", "name", []string{"a", "b", "c"}},
		{"rank", "+name", []string{"b", "c", "a"}},
		{"rank", "-name", []string{"c", "b", "a"}},
		{"", "name", []string{"a", "b", "c"}},
	} {
		items := []sortItem{
			{Name: "c", Rank: 1},
			{Name: "a", Rank: 2},
			{Name: "b", Rank: 1},
		}
		r := Renderer{
			DefaultSort: tc.defaultSort,
			Sort:        tc.sort,
			Output:      "json",
			Data:        items,
		}
		_, err := r.Sprint()
		require.NoErrorf(t, err, "default %q sort %q", tc.defaultSort, tc.sort)
		assert.Equalf(t, tc.expected, names(items), "default %q sort %q", tc.defaultSort, tc.sort)
	}
}
