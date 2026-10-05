package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

func TestAChallengeTokenNamesOnlyItsKey(t *testing.T) {
	k, err := AcmeChallengeKey("LoqXcYV8q5ONbJQxbmR7SCTNo3tiAXDfowyjxAjEuX0")
	require.NoError(t, err)
	assert.Equal(t, "acme_challenge/LoqXcYV8q5ONbJQxbmR7SCTNo3tiAXDfowyjxAjEuX0", k)
	for _, token := range []string{"", "..", "../private_key", "a/b", "a b", "a.b", "a%2Fb"} {
		_, err := AcmeChallengeKey(token)
		assert.Error(t, err, "token %q", token)
	}
}

func TestTheListenerSecsAreNamedByPathOrSystemName(t *testing.T) {
	l := ListenerTLSSecs([]string{"system/sec/public", "web", "ns1/sec/www", "system/cfg/x", "public", ""})
	assert.Equal(t, []naming.Path{
		{Namespace: "system", Kind: naming.KindSec, Name: "public"},
		{Namespace: "system", Kind: naming.KindSec, Name: "web"},
		{Namespace: "ns1", Kind: naming.KindSec, Name: "www"},
	}, l)
}
