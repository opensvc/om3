package uri

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// What the cluster configures is allowed on top of the policy of the node,
// which refuses the rest as it did.
func TestClusterSources(t *testing.T) {
	saved := ClusterSources
	savedURL := rawconfig.SSRFAllowedURL
	defer func() { ClusterSources, rawconfig.SSRFAllowedURL = saved, savedURL }()
	rawconfig.SSRFAllowedURL = nil

	const local = "http://127.0.0.1/a.img"
	for name, tc := range map[string]struct {
		urls, cidrs []string
		allowed     bool
	}{
		"nothing":          {nil, nil, false},
		"the url alone":    {[]string{"http://127.0.0.1/*"}, nil, false},
		"the range alone":  {nil, []string{"127.0.0.0/8"}, false},
		"another url":      {[]string{"https://images.example.com/*"}, []string{"127.0.0.0/8"}, false},
		"the url in range": {[]string{"http://127.0.0.1/*"}, []string{"127.0.0.0/8"}, true},
	} {
		ClusterSources = func() ([]string, []string) { return tc.urls, tc.cidrs }
		_, err := SafeHttpClient(local)
		assert.Equal(t, tc.allowed, err == nil, "%s: %v", name, err)
	}

	ClusterSources = nil
	_, err := SafeHttpClient(local)
	assert.Error(t, err, "a client with no cluster keeps the policy of the node")
}

func TestSharedAddressSpaceIsBlocked(t *testing.T) {
	assert.Contains(t, rawconfig.SSRFBlockedCIDR, "100.64.0.0/10")
}
