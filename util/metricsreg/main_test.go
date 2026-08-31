package metricsreg

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetailNamesArePathSafe guards the one thing the router does with
// these keys: it concatenates them onto /metrics/ to make a route.
func TestDetailNamesArePathSafe(t *testing.T) {
	require.NotEmpty(t, Detail)
	for name, registry := range Detail {
		assert.NotEmpty(t, name)
		assert.NotContains(t, name, "/", "%q would nest a route under another", name)
		assert.NotNil(t, registry)
		_, err := registry.Gather()
		assert.NoError(t, err, "/metrics/%s does not gather", name)
	}
}

// TestDetailRegistriesAreDistinct catches the copy paste that would have
// two endpoints serve the same registry, which reads as a working split
// until someone notices the two pages are identical.
func TestDetailRegistriesAreDistinct(t *testing.T) {
	seen := make(map[*prometheus.Registry]string, len(Detail))
	for name, registry := range Detail {
		if other, ok := seen[registry]; ok {
			t.Errorf("/metrics/%s and /metrics/%s are the same registry", name, other)
		}
		seen[registry] = name
	}
	assert.NotSame(t, prometheus.DefaultRegisterer, Detail["pg"], "the detail must not be the default registry")
}
