package doc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/keyoprbac"
	"github.com/opensvc/om3/v3/core/naming"
)

func TestKeywordGrant(t *testing.T) {
	// A keyword any user may set.
	grant, doc, always := keywordGrant(naming.KindSvc, "DEFAULT", "comment", nil)
	assert.Equal(t, "", grant)
	assert.Equal(t, "", doc)
	assert.False(t, always)

	// A keyword needing the grant whatever its value.
	grant, doc, always = keywordGrant(naming.KindSvc, "task", "run_args", []string{"podman"})
	assert.Equal(t, "root", grant)
	assert.Equal(t, keyoprbac.Doc(naming.KindSvc, "task", "run_args"), doc)
	assert.True(t, always)

	// A keyword needing the grant for some values: the sync types other
	// than store.
	grant, doc, always = keywordGrant(naming.KindSvc, "sync", "type", nil)
	assert.Equal(t, "root", grant)
	assert.Contains(t, doc, "store")
	assert.False(t, always, "the store type needs no grant")

	// A keyword of a driver a user may not choose needs the grant of the
	// driver, whatever its value.
	grant, doc, always = keywordGrant(naming.KindSvc, "sync", "exclude", []string{"rsync"})
	assert.Equal(t, "root", grant)
	assert.Equal(t, "A keyword of the rsync driver, which needs the root grant to choose.", doc)
	assert.True(t, always)

	// A keyword of a driver some user may choose is the keyword's rule.
	grant, _, always = keywordGrant(naming.KindSvc, "sync", "exclude", []string{"store"})
	assert.Equal(t, "", grant)
	assert.False(t, always)
}
