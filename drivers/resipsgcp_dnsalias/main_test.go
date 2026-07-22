package resipsgcp_dnsalias

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/sgcpdnstesthelper"
	"github.com/opensvc/om3/v3/util/testsgcphelper"

	"github.com/opensvc/om3/v3/util/sgcp"
)

// setup initializes the test environment by configuring SGCP with a temporary configuration file.
// It returns a cleanup function that resets the configuration to a null state when invoked.
func setup(t *testing.T) func() {
	t.Helper()
	cfgFile := testsgcphelper.InstallConfig(t)
	sgcp.SetConfigForTest(cfgFile)
	require.NotNil(t, sgcp.GetConfig())

	return func() {
		sgcp.SetConfigForTest("")
	}
}

// newDBAndDrv initializes a database and driver instance for testing with preset
// entries and configurations.
func newDBAndDrv(t *testing.T, s string, entries []sgcpdnstesthelper.DBEntry) (*sgcpdnstesthelper.DB, *T) {
	t.Helper()
	drv := New().(*T)
	require.NoError(t, drv.SetRID(s))
	db := sgcpdnstesthelper.NewDB()
	db.Setup(entries)
	drv.api = sgcpdnstesthelper.NewApi(db)
	return db, drv
}

func TestStatus(t *testing.T) {
	cleanup := setup(t)
	defer cleanup()

	dbEntries := []sgcpdnstesthelper.DBEntry{
		{
			Name:   "svc1",
			UUID:   "a1",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "a1",
					Name:   "svc1",
					Target: "none.xxx",
					FQDN:   "svc1.example.org",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "svc2",
			UUID:   "a2",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "a2",
					Name:   "svc2",
					Target: "tgt2",
					FQDN:   "svc2.example.org",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "svc3",
			UUID:   "a3",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "a3",
					Name:   "svc3bis",
					Target: "node1",
					FQDN:   "svc1.example.org",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "svc4",
			UUID:   "id4",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "svc4-bad-id",
					Name:   "svc4",
					Target: "node1",
					FQDN:   "svc1.example.org",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "multiple",
			UUID:   "multipleId",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "id1",
					Name:   "multiple1",
					Target: "node1",
					FQDN:   "node1.example.org",
					ZoneID: "z1",
				},
				{
					UUID:   "id2",
					Name:   "multiple2",
					Target: "node2",
					FQDN:   "node2.example.org",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:       "bad500",
			ZoneID:     "z1",
			StatusCode: 500,
		},
	}

	cases := map[string]struct {
		resUUID        string
		resName        string
		resZoneID      string
		resTarget      string
		expectedUID    string
		expectedName   string
		expectedZoneID string
		expectedTarget string

		expectedStatus    status.T
		expectedStatusLog []resource.StatusLogEntry
	}{
		"when alias is not found": {
			resName:        "svc1",
			resZoneID:      "z2",
			resTarget:      "node1",
			expectedStatus: status.Down,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "info", Message: "not found"},
			},
		},
		"find alias from name": {
			resName:        "svc2",
			resZoneID:      "z1",
			resTarget:      "tgt2",
			expectedStatus: status.Up,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "info", Message: "svc2.example.org => tgt2"},
			},
		},
		"found target is none": {
			resName:        "svc1",
			resZoneID:      "z1",
			resTarget:      "node1",
			expectedStatus: status.Down,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "info", Message: "alias target is disabled"},
			},
		},
		"target mismatch": {
			resName:        "svc2",
			resZoneID:      "z1",
			resTarget:      "tgtx",
			expectedStatus: status.Down,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "info", Message: "alias target tgt2 != tgtx"},
			},
		},
		"name mismatch": {
			resName:        "svc3",
			resZoneID:      "z1",
			resTarget:      "node1",
			expectedStatus: status.Warn,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "warn", Message: "alias name mismatch: found svc3bis instead of svc3"},
			},
		},
		"uuid mismatch": {
			resUUID:        "id4",
			resName:        "svc4",
			resZoneID:      "z1",
			resTarget:      "node1",
			expectedStatus: status.Warn,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "warn", Message: "alias uuid mismatch: found svc4-bad-id instead of id4"},
			},
		},
		"find multiple aliases": {
			resName:        "multiple",
			resZoneID:      "z1",
			resTarget:      "node1",
			expectedStatus: status.Warn,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "warn", Message: "found multiple aliases: [{id1 multiple1 node1 node1.example.org z1} {id2 multiple2 node2 node2.example.org z1}]"},
			},
		},
		"perfect match": {
			resUUID:        "a2",
			resName:        "svc2",
			resZoneID:      "z1",
			resTarget:      "tgt2",
			expectedStatus: status.Up,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "info", Message: "svc2.example.org => tgt2"},
			},
		},
		"api error 500": {
			resName:        "bad500",
			resZoneID:      "z1",
			resTarget:      "tgt2",
			expectedStatus: status.Down,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "error", Message: "get alias failed: unexpected status code for GET https://dns.example.com/zones/z1/cname-records?name=bad500 got 500 wanted [200 404]"},
				{Level: "info", Message: "not found"},
			},
		},
		"status up when found from only uuid": {
			resUUID:        "a2",
			resZoneID:      "z1",
			resTarget:      "tgt2",
			expectedStatus: status.Up,
			expectedStatusLog: []resource.StatusLogEntry{
				{Level: "info", Message: "svc2.example.org => tgt2"},
			},
		},
	}

	for name, tc := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		t.Run(fmt.Sprintf("%s expect status %s", name, tc.expectedStatus), func(t *testing.T) {
			_, drv := newDBAndDrv(t, "rid1", dbEntries)
			drv.UUID = tc.resUUID
			drv.Name = tc.resName
			drv.Target = tc.resTarget
			drv.ZoneID = tc.resZoneID
			require.NoError(t, drv.Configure())
			drv.mgr.CacheTTL = 0

			dStatus := drv.Status(ctx)
			assert.Equalf(t, tc.expectedStatus, dStatus, "expected %s, got %s", tc.expectedStatus, dStatus)
			statusLog := drv.StatusLog()
			assert.Equal(t, tc.expectedStatusLog, statusLog.Entries())
		})
	}
}

func TestStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	defer setup(t)()

	dbEntries := []sgcpdnstesthelper.DBEntry{
		{
			Name:   "name1",
			UUID:   "uuid1",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "uuid1",
					Name:   "name1",
					Target: "target1",
					FQDN:   "name1.z1",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "name2",
			UUID:   "uuid2",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "uuid2",
					Name:   "name2",
					Target: "target2",
					FQDN:   "name2.z1",
					ZoneID: "z1",
				},
			},
		},
	}

	t.Run("Create missing alias when uuid is unset", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.Name = "foo"
		drv.Target = "foo-target"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify alias doesn't exits")
		alias, ok := db.Search("z1", "foo", "")
		require.Falsef(t, ok, "unexpected found alias")

		db.ResetCalls()
		t.Log("Start will create new alias")
		require.NoError(t, drv.Start(ctx), "unexpected to Start error")
		require.NotEmptyf(t, drv.mgr.alias.UUID, "mgr didn't retrieve alias UUID")
		createdUUID := drv.mgr.alias.UUID
		t.Logf("Created alias uuid: %s", createdUUID)

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 1, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")

		t.Log("verify alias has been created in db")
		alias, ok = db.Search("z1", "foo", "")
		require.True(t, ok)
		assert.NotNil(t, alias)
		assert.Equal(t, createdUUID, alias.UUID)
		assert.Equal(t, "foo", alias.Name)
		t.Logf("created alias: %v", alias)
	})

	t.Run("Don't update alias when all is correct", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.UUID = "uuid1"
		drv.Name = "name1"
		drv.Target = "target1"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify alias initially exits")
		alias, ok := db.Search("z1", "name1", "uuid1")
		require.Truef(t, ok, "didn't find alias")
		t.Logf("initial alias: %v", alias)
		require.NotNil(t, alias)
		assert.Equal(t, "uuid1", alias.UUID, "unexpected initial alias UUID")
		assert.Equal(t, "name1", alias.Name, "unexpected initial alias name")

		db.ResetCalls()
		t.Log("Start don't have to update alias")
		require.NoError(t, drv.Start(ctx), "unexpected to Start error")
		require.Equalf(t, "uuid1", drv.mgr.alias.UUID, "unexpected alias UUID")

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 0, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")
	})

	t.Run("update alias target", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.UUID = "uuid2"
		drv.Name = "name2"
		drv.Target = "newTarget2"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify alias exits initially, with alternate target")
		alias, ok := db.Search("z1", "name2", "uuid2")
		require.Truef(t, ok, "didn't find alias")
		require.NotNil(t, alias)
		t.Logf("initial alias: %v", alias)
		require.Equal(t, drv.UUID, alias.UUID)
		require.Equal(t, drv.Name, alias.Name)
		t.Logf("Existing target for %s is %s", alias.Name, alias.Target)
		require.NotEqual(t, drv.Target, alias.Target)

		db.ResetCalls()
		t.Log("Start must update alias target")
		require.NoError(t, drv.Start(ctx), "unexpected to Start error")
		require.Equalf(t, "newTarget2", drv.mgr.alias.Target, "unexpected alias target")

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 1, call.Update, "unexpected api update call")
		require.Equal(t, 1, call.Search, "unexpected api search call")

		t.Log("verify alias has been created in db")
		alias, ok = db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.True(t, ok)
		require.NotNil(t, alias, "final alias not found")
		t.Logf("final alias: %v", alias)
		require.Equal(t, drv.Target, alias.Target,
			"unexpected final alias target value")
	})
}

func TestStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	defer setup(t)()

	dbEntries := []sgcpdnstesthelper.DBEntry{
		{
			Name:   "name1",
			UUID:   "uuid1",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "uuid1",
					Name:   "name1",
					Target: "target1",
					FQDN:   "name1.z1",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "name2",
			UUID:   "uuid2",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "uuid2",
					Name:   "name2",
					Target: "target2",
					FQDN:   "name2.z1",
					ZoneID: "z1",
				},
			},
		},
		{
			Name:   "name-none",
			UUID:   "uuid-none",
			ZoneID: "z1",
			AliasL: []sgcp.Alias{
				{
					UUID:   "uuid-none",
					Name:   "name-none",
					Target: "none.xxx",
					FQDN:   "name2.z1",
					ZoneID: "z1",
				},
			},
		},
	}

	t.Run("drv with kw uuid: returns error alias is not found", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)

		drv.UUID = "uuid-no-present"
		drv.Name = "no-present-name"
		drv.Target = "target"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify alias doesn't exits")
		_, ok := db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Falsef(t, ok, "unexpected found alias")

		db.ResetCalls()
		t.Log("call Stop")
		err := drv.Stop(ctx)
		t.Logf("error: %v", err)
		require.Error(t, err)
		require.ErrorIsf(t, err, ErrUpdateNotFound, "unexpected to Stop error")

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 0, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")
	})

	t.Run("drv without kw: returns no error if alias is not found", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.Name = "no-present-name"
		drv.Target = "target"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify alias doesn't exits")
		_, ok := db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Falsef(t, ok, "unexpected found alias")

		db.ResetCalls()
		t.Log("call Stop")
		require.NoError(t, drv.Stop(ctx))

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 0, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")
	})

	t.Run("drv with kw uuid must update alias target to none", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.Name = "name1"
		drv.UUID = "uuid1"
		drv.Target = "target1"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify initial exits")
		alias, ok := db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Truef(t, ok, "initial found alias")
		t.Logf("initial alias: %+v", alias)
		require.Equal(t, drv.Target, alias.Target)

		db.ResetCalls()
		t.Log("call Stop")
		require.NoError(t, drv.Stop(ctx))

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 1, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")

		t.Log("verify alias has been updated")
		alias, ok = db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Truef(t, ok, "final found alias")
		t.Logf("final alias: %+v", alias)
		require.Equal(t, drv.noneTarget, alias.Target)
	})

	t.Run("drv with kw uuid and target already none is noop", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.Name = "name-none"
		drv.UUID = "uuid-none"
		drv.Target = "none.xxx"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify initial exits with target none")
		alias, ok := db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Truef(t, ok, "initial found alias")
		t.Logf("initial alias: %+v", alias)
		require.Equal(t, drv.noneTarget, alias.Target)

		db.ResetCalls()
		t.Log("call Stop")
		require.NoError(t, drv.Stop(ctx))

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 0, call.Delete, "unexpected api delete call")
		require.Equal(t, 0, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")

		t.Log("verify alias has not been updated")
		alias, ok = db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Truef(t, ok, "final found alias")
		t.Logf("final alias: %+v", alias)
		require.Equal(t, drv.noneTarget, alias.Target)
	})

	t.Run("drv without kw uuid must delete alias", func(t *testing.T) {
		db, drv := newDBAndDrv(t, "rid1", dbEntries)
		drv.Name = "name1"
		drv.Target = "target1"
		drv.ZoneID = "z1"
		require.NoError(t, drv.Configure())
		drv.mgr.CacheTTL = 0

		t.Log("verify initial exits")
		initial, ok := db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Truef(t, ok, "initial found alias")
		t.Logf("initial alias: %+v", initial)
		require.Equal(t, drv.Target, initial.Target)

		db.ResetCalls()
		t.Log("call Stop")
		require.NoError(t, drv.Stop(ctx))

		call := db.CallCounts()
		t.Logf("call counts: %+v", call)
		require.Equal(t, 1, call.Delete, "unexpected api delete call")
		require.Equal(t, 0, call.Update, "unexpected api update call")
		assert.Equal(t, 1, call.Search, "unexpected api search call")

		t.Log("verify alias has been deleted")
		final, ok := db.Search(drv.ZoneID, drv.Name, drv.UUID)
		require.Falsef(t, ok, "found alias")
		t.Logf("final alias: %+v", final)
	})
}
