package ccfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A network section removed is reported, and its signature dropped, so the
// network added back with the same configuration is reported again.
func TestNetworkSectionsChangedReportsARemoval(t *testing.T) {
	cfg := map[string]string{
		"network#a": "sig-a",
		"network#b": "sig-b",
		"node":      "sig-node",
	}
	sections := func() []string {
		l := make([]string, 0, len(cfg))
		for name := range cfg {
			l = append(l, name)
		}
		return l
	}
	sigOf := func(name string) string { return cfg[name] }
	sigs := make(map[string]string)

	assert.Equal(t, []string{"network#a", "network#b"}, networkSectionsChanged(sigs, sections(), sigOf))
	assert.Empty(t, networkSectionsChanged(sigs, sections(), sigOf))

	delete(cfg, "network#b")
	assert.Equal(t, []string{"network#b"}, networkSectionsChanged(sigs, sections(), sigOf))
	assert.NotContains(t, sigs, "network#b")
	assert.Empty(t, networkSectionsChanged(sigs, sections(), sigOf))

	cfg["network#b"] = "sig-b"
	assert.Equal(t, []string{"network#b"}, networkSectionsChanged(sigs, sections(), sigOf))
}
