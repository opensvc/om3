package resiphost_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"

	_ "github.com/opensvc/om3/v3/drivers/resiphost"
)

// TestAddrIsNotCloned pins that a configuration copied to make another object
// leaves the address drawn behind: the clone draws its own, where it would
// otherwise bring the address of the source up a second time.
func TestAddrIsNotCloned(t *testing.T) {
	p, err := naming.ParsePath("test/svc/clone")
	require.NoError(t, err)
	o, err := object.New(p, object.WithConfigData([]byte(`
[ip#0]
type = host
network = san
addr = fd01:2345:6789:2902::5:e8
`)), object.WithVolatile(true))
	require.NoError(t, err)
	cfg := o.(object.Configurer).Config()
	cfg.UnsetRecorded()
	assert.Empty(t, cfg.GetString(key.New("ip#0", "addr")))
	assert.Equal(t, "san", cfg.GetString(key.New("ip#0", "network")), "what is not recorded is copied")
}
