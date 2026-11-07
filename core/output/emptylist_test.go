package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type emptyListItem struct {
	Name string `json:"name"`
}

// emptyList is shaped as the lists the api answers with.
type emptyList struct {
	Items []emptyListItem `json:"items"`
	Kind  string          `json:"kind"`
}

// A listing of nothing is an empty list in the machine formats, not null,
// which a script can not iterate: a slice never allocated, and the list of
// an api answer whose items were never allocated, as the execs of no node.
func TestAnEmptyListingIsAnEmptyList(t *testing.T) {
	render := func(out string, data any) string {
		t.Helper()
		s, err := Renderer{Output: out, Data: data}.Sprint()
		require.NoError(t, err)
		return s
	}
	assert.JSONEq(t, `{"items": [], "kind": "ExecList"}`, render("json", emptyList{Kind: "ExecList"}))
	assert.JSONEq(t, `{"items": [], "kind": "ExecList"}`, render("jsonline", emptyList{Kind: "ExecList"}))
	assert.Contains(t, render("yaml", emptyList{Kind: "ExecList"}), "items: []")
	assert.NotContains(t, render("flat", emptyList{Kind: "ExecList"}), "null")
	assert.JSONEq(t, `[]`, render("json", []emptyListItem(nil)))

	p := &emptyList{Kind: "ExecList"}
	assert.JSONEq(t, `{"items": [], "kind": "ExecList"}`, render("json", p))
	assert.Nil(t, p.Items, "the list rendered is not changed")

	full := emptyList{Kind: "ExecList", Items: []emptyListItem{{Name: "a"}}}
	assert.JSONEq(t, `{"items": [{"name": "a"}], "kind": "ExecList"}`, render("json", full))
}
