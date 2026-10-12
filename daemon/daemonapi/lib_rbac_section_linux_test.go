//go:build linux

package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The policy judges a section by its driver group, so the rest of its name is
// checked apart: a flag is open to the namespace administrator, a flag kept
// outside the var directory of the object, or named after a command, is not.
func TestASectionNamedWithNoResourceIdCharacterIsRefused(t *testing.T) {
	knownNodes(t)
	for _, section := range []string{"fs#x/../../../../etc/cron.d", "fs#$(id)", "fs#a b"} {
		assert.Error(t, sectionNameRbac(configOf(t, "[DEFAULT]\nnodes = *\n\n["+section+"]\ntype = flag\n")), section)
	}
	assert.NoError(t, sectionNameRbac(configOf(t, `[DEFAULT]
nodes = *

[fs#1]
type = flag
subset = fs:g1

[subset#fs:g1]
parallel = true
`)), "a subset is named after its group and its name")
}
