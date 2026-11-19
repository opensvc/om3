package daemonapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

func setupCreateTombstone(t *testing.T) (naming.Path, time.Time) {
	t.Helper()
	tombstone.InitData()
	instance.InitData()
	node.InitData()
	t.Cleanup(func() {
		tombstone.InitData()
		instance.InitData()
		node.InitData()
	})
	clusternode.Set([]string{"n1", "n2", "n3"})
	for _, n := range []string{"n1", "n2", "n3"} {
		node.MonitorData.Set(n, &node.Monitor{State: node.MonitorStateIdle})
	}
	version := time.Date(2026, 11, 17, 15, 33, 47, 0, time.UTC)
	p := naming.Path{Namespace: "tombtest", Kind: naming.KindSvc, Name: "c1"}
	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: p.String(), Kind: tombstone.KindPurged, ObjectID: "o1", Version: version}, true)
	return p, version
}

// A create of another object under a path whose deleted version a node
// holds, or a node away may hold, is refused; the same object, restored, is
// not, nor any create once no node may hold it.
func TestTombstoneCreateRefusal(t *testing.T) {
	p, version := setupCreateTombstone(t)
	assert.Empty(t, tombstoneCreateRefusal(p, "o2"), "every node up, none holding the version deleted")

	instance.ConfigData.Set(p, "n3", &instance.Config{Path: p, UpdatedAt: version})
	assert.Contains(t, tombstoneCreateRefusal(p, "o2"), "n3 still holds its instance")
	assert.Contains(t, tombstoneCreateRefusal(p, ""), "n3 still holds", "an id unknown is another")
	assert.Empty(t, tombstoneCreateRefusal(p, "o1"), "a restore of the same object")

	instance.ConfigData.Set(p, "n3", &instance.Config{Path: p, UpdatedAt: version.Add(time.Second)})
	assert.Empty(t, tombstoneCreateRefusal(p, "o2"), "n3 kept a newer version: no holder")

	instance.ConfigData.Unset(p, "n3")
	node.MonitorData.Set("n3", &node.Monitor{State: node.MonitorStateMaintenance})
	assert.Contains(t, tombstoneCreateRefusal(p, "o2"), "n3 away", "a record listing no node: any node away")

	tombstone.Data.Set("n1", tombstone.Record{OrchestrationID: "a", Path: p.String(), Kind: tombstone.KindPurged, ObjectID: "o1", Version: version, Nodes: []string{"n2"}}, true)
	assert.Empty(t, tombstoneCreateRefusal(p, "o2"), "n3 away is not a node the record lists")
	node.MonitorData.Set("n2", &node.Monitor{State: node.MonitorStateMaintenance})
	assert.Contains(t, tombstoneCreateRefusal(p, "o2"), "n2 away")
}

// The refusal reaches the client as a conflict.
func TestCreateRefusedOverATombstone(t *testing.T) {
	p, version := setupCreateTombstone(t)
	instance.ConfigData.Set(p, "n3", &instance.Config{Path: p, UpdatedAt: version})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("[DEFAULT]\n"))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("logger", plog.NewDefaultLogger())
	c.Set("grants", rbac.Grants{rbac.GrantRoot})
	require.NoError(t, (&DaemonAPI{localhost: "n1"}).PostObjectConfigFile(c, p.Namespace, p.Kind, p.Name, api.PostObjectConfigFileParams{}))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "n3 still holds its instance")
}
