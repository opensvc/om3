package object_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"

	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// A keyword the provision needs is one the provision reads, and has no
// default: a default the provision works with makes it not needed.
func TestProvisioningRequiredIsAProvisioningKeywordWithNoDefault(t *testing.T) {
	n := 0
	for _, kind := range []naming.Kind{naming.KindSvc, naming.KindVol} {
		for _, kw := range object.KeywordStoreWithDrivers(kind) {
			if !kw.ProvisioningRequired {
				continue
			}
			n++
			name := kw.Section + "." + kw.Option
			assert.True(t, kw.Provisioning, "%s %v: needed by the provision, it must be a provisioning keyword", name, kw.Types)
			assert.Empty(t, kw.Default, "%s %v: a default the provision works with makes it not needed", name, kw.Types)
			assert.False(t, kw.Required, "%s %v: required already says it is needed, to run as to provision", name, kw.Types)
		}
	}
	assert.NotZero(t, n, "the drivers must be registered")
}
