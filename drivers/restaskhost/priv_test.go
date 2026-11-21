package restaskhost

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A host task declares the privilege restriction keywords of the app
// drivers, and passes them to the app it runs its command as.
func TestPrivKeywords(t *testing.T) {
	r := &T{NoNewPrivileges: "true", CapabilityBoundingSet: "CAP_CHOWN"}
	options := make(map[string]bool)
	for _, kw := range r.Manifest().Keywords() {
		options[kw.Option] = true
	}
	assert.True(t, options["no_new_privileges"])
	assert.True(t, options["capability_bounding_set"])
	app := r.App()
	assert.Equal(t, "true", app.NoNewPrivileges)
	assert.Equal(t, "CAP_CHOWN", app.CapabilityBoundingSet)
}
