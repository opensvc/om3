package msgbus

import (
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEveryKindHasAText pins that every event kind says when it is published,
// and that no text is left of a kind gone: a new kind comes with its text in
// text/kind, as events doc shows it.
func TestEveryKindHasAText(t *testing.T) {
	kinds := Kinds()
	for _, kind := range kinds {
		text, ok := Text(kind)
		if assert.Truef(t, ok, "%s has no text in daemon/msgbus/text/kind", kind) {
			assert.NotEmptyf(t, Doc{Text: text}.Summary(), "%s has no summary line", kind)
		}
	}
	entries, err := fs.ReadDir(texts, "text/kind")
	require.NoError(t, err)
	for _, e := range entries {
		assert.Containsf(t, kinds, e.Name(), "text/kind/%s is the text of no event kind", e.Name())
	}
}

func TestGetDoc(t *testing.T) {
	doc, err := GetDoc("NodeNetworkStatusUpdated")
	require.NoError(t, err)
	assert.Equal(t, "A node set up a network.", doc.Summary())
	assert.Equal(t, []Field{
		{Name: "node", Type: "string"},
		{Name: "name", Type: "string"},
		{Name: "network_status", Type: "netstatus.Status"},
	}, doc.Fields)

	_, err = GetDoc("NoSuchKind")
	assert.ErrorContains(t, err, "unknown event kind NoSuchKind")
}
