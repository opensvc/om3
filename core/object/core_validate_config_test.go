package object

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

// The snapshot syncs of an object whose schedule differs from the first's
// are warned of: the scheduler runs together the syncs due at once only, so
// their snapshots are taken in separate runs. A sync with delay_snap, with no
// snap, or of another type, is not grouped, and not warned of.
func TestSnapScheduleAlerts(t *testing.T) {
	testhelper.Setup(t)
	clusterConfig := cluster.Config{Name: "cluster1"}
	clusterConfig.SetSecret("9ceab2da-a126-4187-83f2-4900da8a6825")
	cluster.ConfigData.Set(&clusterConfig)
	p, err := naming.ParsePath("test/svc/snapsched")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0o755))
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte(`[sync#1]
type = rsync
snap = true
schedule = @1h

[sync#2]
type = rsync
snap = true
schedule = @1h

[sync#3]
type = rsync
snap = true
schedule = @1d

[sync#4]
type = rsync
snap = true
delay_snap = true
schedule = @1d

[sync#5]
type = rsync
schedule = @1d

[sync#6]
type = zfs
src = pool/a
dst = pool/b
schedule = @1d
`), 0o644))
	o, err := New(p)
	require.NoError(t, err)
	alerts := snapScheduleAlerts(o.(*svc).config)
	require.Len(t, alerts, 1, "%v", alerts)
	assert.Equal(t, "sync#3.schedule", alerts[0].Key.String())
	assert.Contains(t, alerts[0].Comment, "differs from sync#1.schedule")
	assert.Contains(t, alerts[0].Comment, "delay_snap = true")
}
