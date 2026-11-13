package instance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

// An object uses the objects it names, and the ones a pattern of their name
// it names matches, in their namespace, the root one included.
func TestActorConfigUsesObject(t *testing.T) {
	cfg := &ActorConfig{Uses: []string{"ns1/cfg/d", "ns1/sec/web-*", "sec/ca", "ns1/vol/d-data"}}
	path := func(s string) naming.Path {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		return p
	}
	assert.True(t, cfg.UsesObject(path("ns1/cfg/d")))
	assert.True(t, cfg.UsesObject(path("ns1/vol/d-data")))
	assert.True(t, cfg.UsesObject(path("ns1/sec/web-front")), "by a pattern")
	assert.True(t, cfg.UsesObject(path("sec/ca")), "of the root namespace")
	assert.False(t, cfg.UsesObject(path("ns1/sec/d")), "another kind")
	assert.False(t, cfg.UsesObject(path("ns2/cfg/d")), "another namespace")
	assert.False(t, cfg.UsesObject(path("ns2/sec/web-front")), "a pattern of another namespace")
	assert.False(t, cfg.UsesObject(path("ns1/sec/web-")), "a * matches one character or more")
	assert.False(t, (*ActorConfig)(nil).UsesObject(path("ns1/cfg/d")))

	b, err := json.Marshal(Config{ActorConfig: cfg})
	require.NoError(t, err)
	var back Config
	require.NoError(t, json.Unmarshal(b, &back))
	assert.Equal(t, cfg.Uses, back.Uses, "published with the instance configuration")
	assert.Equal(t, cfg.Uses, (&Config{ActorConfig: cfg}).DeepCopy().Uses)
}
