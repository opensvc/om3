package daemonapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/netwarden"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/plog"
)

func getNetworkAddress(t *testing.T, grants rbac.Grants, params api.GetNetworkAddressParams) (*httptest.ResponseRecorder, api.NetworkAddressList) {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	ctx.Set("grants", grants)
	ctx.Set("logger", plog.NewDefaultLogger())
	a := &DaemonAPI{localhost: "node1"}
	require.NoError(t, a.GetNetworkAddress(ctx, params))
	var list api.NetworkAddressList
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	}
	return rec, list
}

// The addresses of the objects are what the rules of net_rules
// naming them resolve to: the union of what their instances publish, on all
// the nodes, by resource. They say how the objects of any namespace are
// reached, so root and the inspector read them, and nobody else.
func TestGetNetworkAddress(t *testing.T) {
	instance.InitData()
	cluster.InitData()
	cluster.ConfigData.Set(&cluster.Config{Nodes: []string{"node1", "node2"}})
	t.Cleanup(func() {
		instance.InitData()
		cluster.InitData()
	})
	set := func(s, nodename string, addrs ...string) {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		info := map[string]any{}
		if len(addrs) > 0 {
			info[resource.IPAddrInfoKey] = addrs[0]
			info[resource.IPAddrsInfoKey] = addrs
		}
		instance.ConfigData.Set(p, nodename, &instance.Config{})
		instance.StatusData.Set(p, nodename, &instance.Status{Resources: instance.ResourceStatuses{
			"ip#1": resource.Status{Status: status.Up, Info: info},
			"fs#1": resource.Status{Status: status.Up},
		}})
	}
	set("test/svc/db", "node1", "10.0.1.5")
	set("test/svc/db", "node2", "10.0.1.5", "fd00::5")
	set("prod/svc/db", "node2", "10.0.2.5")
	set("prod/svc/down", "node2")
	set("other/svc/db", "node1", "10.0.3.5")

	objects := []string{"test/svc/db", "test/svc/absent"}
	namespaces := []string{"prod", "nowhere"}
	params := api.GetNetworkAddressParams{Object: &objects, Namespace: &namespaces}

	for _, grants := range []rbac.Grants{
		{},
		{rbac.NewGrant(rbac.RoleAdmin, "test")},
		{rbac.NewGrant(rbac.RoleGuest, "prod")},
		{rbac.GrantSquatter},
	} {
		rec, _ := getNetworkAddress(t, grants, params)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%v", grants)
		assert.NotContains(t, rec.Body.String(), "10.0.", "%v", grants)
	}

	for _, grants := range []rbac.Grants{{rbac.GrantRoot}, {rbac.GrantInspector}} {
		rec, list := getNetworkAddress(t, grants, params)
		require.Equal(t, http.StatusOK, rec.Code, "%v", grants)
		// No configuration tells an id or a version here: no address
		// is told an object, in a list told all the same.
		none := &[]api.NetworkAddressHolder{}
		assert.Equal(t, api.NetworkAddressList{
			Kind: "NetworkAddressList",
			Items: api.NetworkAddressItems{
				{Path: "prod/svc/db", Holders: none, Resources: map[string][]string{"ip#1": {"10.0.2.5"}, "fs#1": {}}},
				{Path: "prod/svc/down", Holders: none, Resources: map[string][]string{"ip#1": {}, "fs#1": {}}},
				{Path: "test/svc/db", Holders: none, Resources: map[string][]string{"ip#1": {"10.0.1.5", "fd00::5"}, "fs#1": {}}},
			},
		}, list, "%v: the objects asked for and no other, sorted, the absent ones left out", grants)
	}

	rec, list := getNetworkAddress(t, rbac.Grants{rbac.GrantRoot}, api.GetNetworkAddressParams{})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, list.Items, "nothing asked, nothing told")
	assert.Contains(t, rec.Body.String(), `"items":[]`)

	bad := []string{"a/b/c/d"}
	rec, _ = getNetworkAddress(t, rbac.Grants{rbac.GrantRoot}, api.GetNetworkAddressParams{Object: &bad})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Each object is told with its id and the version of its configuration, the
// newest of the instances publishing its addresses: the start of a resource
// records which object published an address, where the path alone would
// name the object deleted and the one created again under it alike. An
// object whose configuration tells neither is told without them, as a
// daemon knowing no such field tells every object.
func TestGetNetworkAddressTellsWhichObjectEachPathNames(t *testing.T) {
	instance.InitData()
	cluster.InitData()
	cluster.ConfigData.Set(&cluster.Config{Nodes: []string{"node1", "node2"}})
	t.Cleanup(func() {
		instance.InitData()
		cluster.InitData()
	})
	older := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	set := func(s, nodename string, cfg *instance.Config) {
		p, err := naming.ParsePath(s)
		require.NoError(t, err)
		instance.ConfigData.Set(p, nodename, cfg)
		instance.StatusData.Set(p, nodename, &instance.Status{Resources: instance.ResourceStatuses{
			"ip#1": resource.Status{Status: status.Up, Info: map[string]any{resource.IPAddrInfoKey: "10.0.1.5"}},
		}})
	}
	set("test/svc/db", "node1", &instance.Config{ID: "old-id", UpdatedAt: older})
	set("test/svc/db", "node2", &instance.Config{ID: "new-id", UpdatedAt: newer})
	set("test/svc/anon", "node1", &instance.Config{})

	// An object created again, its new configuration told by a node
	// publishing no address yet, while another still publishes the
	// address of the object deleted: the addresses are the ones of the
	// object deleted.
	set("test/svc/moved", "node1", &instance.Config{ID: "old-id", UpdatedAt: older})
	moved, err := naming.ParsePath("test/svc/moved")
	require.NoError(t, err)
	instance.ConfigData.Set(moved, "node2", &instance.Config{ID: "new-id", UpdatedAt: newer})
	instance.StatusData.Set(moved, "node2", &instance.Status{Resources: instance.ResourceStatuses{
		"ip#1": resource.Status{Status: status.Down},
	}})

	objects := []string{"test/svc/db", "test/svc/anon", "test/svc/moved"}
	rec, list := getNetworkAddress(t, rbac.Grants{rbac.GrantRoot}, api.GetNetworkAddressParams{Object: &objects})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, list.Items, 3)

	anon, db := list.Items[0], list.Items[1]
	require.NotNil(t, list.Items[2].ObjectId)
	require.NotNil(t, list.Items[2].Version)
	assert.Equal(t, "old-id", *list.Items[2].ObjectId, "the id of the object publishing the address")
	assert.True(t, older.Equal(*list.Items[2].Version), "the version of the configuration of the instance publishing the address, got %s", list.Items[2].Version)
	assert.Equal(t, "test/svc/anon", anon.Path)
	assert.Nil(t, anon.ObjectId, "no id known, none told")
	assert.Nil(t, anon.Version, "no version known, none told")

	assert.Equal(t, "test/svc/db", db.Path)
	require.NotNil(t, db.ObjectId)
	require.NotNil(t, db.Version)
	assert.Equal(t, "new-id", *db.ObjectId, "the id of the newest configuration of the instances publishing the address")
	assert.True(t, newer.Equal(*db.Version), "the version of the newest configuration of the instances publishing the address, got %s", db.Version)

	id, version := netwarden.Identity(naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "db"})
	assert.Equal(t, *db.ObjectId, id, "what the daemon keeping the filters current weighs a record of a delete against")
	assert.True(t, version.Equal(*db.Version))

	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.NotContains(t, raw.Items[0], "object_id")
	assert.NotContains(t, raw.Items[0], "version")
	assert.Contains(t, raw.Items[1], "object_id")
	assert.Contains(t, raw.Items[1], "version")
}

// Each address is told with the object publishing it: a path names two
// objects while a node back from a delete still tells its instance of the
// object deleted, publishing its address, beside the object created again
// under the path, publishing another. The object told for the path, which
// an om knowing no object by address reads, is the newest of them.
func TestGetNetworkAddressTellsWhichObjectPublishesEachAddress(t *testing.T) {
	instance.InitData()
	cluster.InitData()
	cluster.ConfigData.Set(&cluster.Config{Nodes: []string{"node1", "node2", "node3"}})
	t.Cleanup(func() {
		instance.InitData()
		cluster.InitData()
	})
	older := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	db, err := naming.ParsePath("test/svc/db")
	require.NoError(t, err)
	set := func(nodename string, cfg *instance.Config, addrs ...string) {
		if cfg != nil {
			instance.ConfigData.Set(db, nodename, cfg)
		}
		instance.StatusData.Set(db, nodename, &instance.Status{Resources: instance.ResourceStatuses{
			"ip#1": resource.Status{Status: status.Up, Info: map[string]any{resource.IPAddrsInfoKey: addrs}},
		}})
	}
	set("node1", &instance.Config{ID: "new-id", UpdatedAt: newer}, "10.0.1.6", "fd00::6", "10.0.1.7")
	set("node2", &instance.Config{ID: "old-id", UpdatedAt: older}, "10.0.1.5", "10.0.1.7")
	// An instance whose configuration is not known: its address is told
	// no object, not the one of the path.
	set("node3", nil, "10.0.1.8")

	objects := []string{"test/svc/db"}
	rec, list := getNetworkAddress(t, rbac.Grants{rbac.GrantRoot}, api.GetNetworkAddressParams{Object: &objects})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, list.Items, 1)
	item := list.Items[0]
	assert.ElementsMatch(t, []string{"10.0.1.5", "10.0.1.6", "10.0.1.7", "10.0.1.8", "fd00::6"}, item.Resources["ip#1"])

	require.NotNil(t, item.Holders)
	holders := *item.Holders
	require.Len(t, holders, 2, "an object per configuration publishing, the address of no known one in none: %s", rec.Body.String())
	require.NotNil(t, holders[0].ObjectId)
	require.NotNil(t, holders[0].Version)
	assert.Equal(t, "old-id", *holders[0].ObjectId)
	assert.True(t, older.Equal(*holders[0].Version))
	assert.Equal(t, []string{"10.0.1.5"}, holders[0].Addresses, "the address the instance of the object deleted publishes alone")
	require.NotNil(t, holders[1].ObjectId)
	require.NotNil(t, holders[1].Version)
	assert.Equal(t, "new-id", *holders[1].ObjectId)
	assert.True(t, newer.Equal(*holders[1].Version))
	assert.Equal(t, []string{"10.0.1.6", "10.0.1.7", "fd00::6"}, holders[1].Addresses, "its own, and the one both publish, which is of the newest")

	require.NotNil(t, item.ObjectId)
	require.NotNil(t, item.Version)
	assert.Equal(t, "new-id", *item.ObjectId, "the object of the path, for an om knowing no holder")
	assert.True(t, newer.Equal(*item.Version))
}
