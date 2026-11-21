package resappsimple

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The driver declares the privilege restriction keywords of the app
// drivers.
func TestPrivKeywords(t *testing.T) {
	options := make(map[string]bool)
	for _, kw := range (&T{}).Manifest().Keywords() {
		options[kw.Option] = true
	}
	assert.True(t, options["no_new_privileges"])
	assert.True(t, options["capability_bounding_set"])
}
