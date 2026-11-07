package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/output"
)

type listingItem struct {
	Ref string `json:"ref"`
}

// A listing a command builds prints {kind, items} in the machine formats,
// an empty one [], and its rows in a table, sorted as asked.
func TestAListingBuiltIsAListInTheMachineFormats(t *testing.T) {
	render := func(out, sort string, data any) string {
		t.Helper()
		s, err := output.Renderer{DefaultOutput: "tab=REF:ref", Output: out, Sort: sort, Data: data}.Sprint()
		require.NoError(t, err)
		return s
	}
	l := NewList("StoreList", []listingItem{{Ref: "b"}, {Ref: "a"}})
	assert.JSONEq(t, `{"kind": "StoreList", "items": [{"ref": "a"}, {"ref": "b"}]}`, render("json", "", l), "sorted as the table")
	assert.JSONEq(t, `{"kind": "StoreList", "items": []}`, render("json", "", NewList[listingItem]("StoreList", nil)))
	assert.Equal(t, "REF\na\nb\n", stripSpaces(render("tab=REF:ref", "REF", l)))
}

func stripSpaces(s string) string {
	out := []rune{}
	for _, r := range s {
		if r != ' ' {
			out = append(out, r)
		}
	}
	return string(out)
}
