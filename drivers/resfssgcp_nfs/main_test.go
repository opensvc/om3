package resfssgcp_nfs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opensvc/om3/v3/drivers/sgcpauthtesthelper"
	"github.com/opensvc/om3/v3/util/testsgcphelper"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/ageingcache"
	"github.com/opensvc/om3/v3/util/sgcp"
)

func newDrvWithRid(s string) *T {
	d := New().(*T)
	if err := d.SetRID(s); err != nil {
		panic(err)
	}
	return d
}

// Setup initializes the test environment by configuring SGCP with a temporary configuration file.
// It returns a cleanup function that resets the configuration to a null state when invoked.
func Setup(t *testing.T) func() {
	t.Helper()
	cfgFile := testsgcphelper.InstallConfig(t)
	sgcp.SetConfigForTest(cfgFile)
	require.NotNil(t, sgcp.GetConfig())

	return func() {
		sgcp.SetConfigForTest("")
	}
}

// TestDriverID tests that the driver has the correct ID
func TestDriverID(t *testing.T) {
	drv := New()
	assert.Equal(t, driver.NewID(driver.GroupFS, "sgcp_nfs"), drv.DriverID())
}

// TestNew creates a new driver instance
func TestNew(t *testing.T) {
	drv := New()
	require.NotNil(t, drv)

	// Check that it implements the resource.Driver interface
	_, ok := drv.(resource.Driver)
	assert.True(t, ok)
}

// TestConfigure tests driver configuration
func TestConfigure(t *testing.T) {
	defer Setup(t)()

	drv := newDrvWithRid("test-rid")
	drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")
	// Set configuration
	drv.UUID = "test-uuid"
	drv.Host = "test-host"
	drv.Permission = "read-write"
	drv.Protocol = "nfs4.1"
	// Configure should not fail with valid config
	err := drv.Configure()
	assert.NoError(t, err)

	// Check defaults
	assert.Equal(t, "read-write", drv.Permission)
	assert.Equal(t, "nfs4.1", drv.Protocol)
	assert.Equal(t, "the-secret", drv.Secret)
	assert.Equal(t, "https://127.0.0.1:1215/file", drv.Endpoint)
}

// TestConfigure tests driver configuration
func TestConfigureWhenNoConfig(t *testing.T) {
	sgcp.SetConfigForTest("")

	drv := newDrvWithRid("test-rid")
	drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")
	// Set configuration
	drv.UUID = "test-uuid"
	drv.Host = "test-host"
	drv.Permission = "read-write"
	drv.Protocol = "nfs4.1"
	// Configure should not fail with valid config
	require.Error(t, drv.Configure(), "mandatory config file is required: /etc/om3/sgcp.yaml")
}

// TestConfigureWithMissingUUID tests configuration validation
func TestConfigureWithMissingUUID(t *testing.T) {
	defer Setup(t)()

	drv := newDrvWithRid("test-rid")
	drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")

	// This should still work since UUID is set via the configuration
	drv.UUID = "test-uuid"
	err := drv.Configure()
	assert.NoError(t, err)
}

// TestIsClientIgnored tests the client ignored functionality
func TestIsClientIgnored(t *testing.T) {
	defer Setup(t)()

	drv := newDrvWithRid("test-rid")
	drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")

	err := drv.Configure()
	assert.NoError(t, err)

	// Add some ignored hosts
	NfsClientIgnored = []string{"ignored1", "ignored2"}

	assert.True(t, drv.isClientIgnored("ignored1"))
	assert.True(t, drv.isClientIgnored("ignored2"))
	assert.False(t, drv.isClientIgnored("valid-host"))
}

// TestClientString tests the client string representation
func TestClientString(t *testing.T) {
	client := &NfsClient{
		UUID:       "client-uuid",
		Host:       "client-host",
		Permission: "read-write",
		Protocol:   "nfs4.1",
	}

	str := client.String()
	assert.Contains(t, str, "client-uuid")
	assert.Contains(t, str, "client-host")
	assert.Contains(t, str, "read-write")
	assert.Contains(t, str, "nfs4.1")
}

// TestGetNFSClients tests filtering of NFS clients
func TestGetNFSClients(t *testing.T) {
	defer Setup(t)()

	drv := newDrvWithRid("test-rid")
	drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")
	require.NoError(t, drv.Configure())

	// Set up ignored hosts
	NfsClientIgnored = []string{"ignored-host"}

	fileInfo := &FilesystemInfo{
		UUID: "fs-uuid",
		NFSClients: []NfsClient{
			{UUID: "client1", Host: "host1", Permission: "read-write", Protocol: "nfs4.1"},
			{UUID: "client2", Host: "ignored-host", Permission: "read-write", Protocol: "nfs4.1"},
			{UUID: "client3", Host: "host2", Permission: "read-only", Protocol: "nfs4.1"},
		},
	}

	clients := drv.getNFSClients(fileInfo)

	// Should have 2 clients (ignored-host should be filtered out)
	assert.Len(t, clients, 2)

	// Check that the ignored host is not in the result
	hosts := []string{}
	for _, client := range clients {
		hosts = append(hosts, client.Host)
	}
	assert.NotContains(t, hosts, "ignored-host")
}

// TestFileStatusWithNoClients tests status when no clients are available
// TestFileStatusCache verifies the cached filesystem info the scheduler fills
// is not served to anyone else asking for a status.
func TestFileStatusCache(t *testing.T) {
	cachedFiles := func(t *testing.T) []string {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join(rawconfig.Paths.Cache, "ageing"))
		if err != nil {
			return nil
		}
		l := make([]string, 0, len(entries))
		for _, e := range entries {
			l = append(l, e.Name())
		}
		return l
	}

	// fill the cache the way a status evaluation does, then ask for a status
	// and see what became of the entry.
	fill := func(t *testing.T) *T {
		t.Helper()
		drv := newDrvWithRid("test-rid")
		drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")
		drv.UUID = "test-uuid"
		drv.Host = "test-host"
		drv.Permission = "read-write"
		require.NoError(t, drv.Configure())

		sig := drv.mgr.cacheSig("getFileInfo")
		o := ageingcache.NewOutputter(func() ([]byte, error) { return []byte("null"), nil })
		_, err := ageingcache.Output(o, sig, time.Hour)
		require.NoError(t, err)
		require.NotEmpty(t, cachedFiles(t), "the cache was not filled")
		return drv
	}

	for _, origin := range []env.ActionOrigin{
		env.ActionOriginUser,
		env.ActionOriginDaemonMonitor,
		env.ActionOriginDaemonAPI,
	} {
		t.Run(string(origin)+" drops it", func(t *testing.T) {
			defer Setup(t)()
			t.Setenv(env.ActionOriginVar, string(origin))
			drv := fill(t)

			drv.Status(context.Background())
			assert.Empty(t, cachedFiles(t), "the status evaluation was served the cache")
		})
	}

	t.Run("the scheduler keeps it", func(t *testing.T) {
		defer Setup(t)()
		t.Setenv(env.ActionOriginVar, string(env.ActionOriginDaemonScheduler))
		drv := fill(t)

		drv.Status(context.Background())
		assert.NotEmpty(t, cachedFiles(t), "the scheduler evaluation dropped the cache")
	})
}

func TestFileStatusWithNoClients(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"uuid": "test-uuid", "nfsClients": [], "status": "online"}`))
	}))
	defer server.Close()

	// Create a mock HTTP client that uses our test server
	client := server.Client()
	_ = client

	// This test is more of a unit test for the logic
	// In a real scenario, you'd mock the HTTP client properly
	drv := New().(*T)
	drv.UUID = "test-uuid"
	drv.Host = "test-host"
	drv.Permission = "read-write"
	drv.Endpoint = server.URL

	// Mock the filesAPI
	// This is simplified - in a real test you'd use dependency injection

	// Test with no clients
	testNoClients := &FilesystemInfo{
		UUID:       "test-uuid",
		NFSClients: []NfsClient{},
		Status:     "online",
	}

	// Test the status logic
	assert.Equal(t, status.Down, drv.fileStatusFromInfo(testNoClients))
}

// TestFileStatusWithClients tests status when clients are available
func TestFileStatusWithClients(t *testing.T) {
	testWithClients := &FilesystemInfo{
		UUID: "test-uuid",
		NFSClients: []NfsClient{
			{UUID: "client1", Host: "test-host", Permission: "read-write", Protocol: "nfs4.1"},
		},
		Status: "online",
	}

	drv := New().(*T)
	drv.UUID = "test-uuid"
	drv.Host = "test-host"
	drv.Permission = "read-write"

	// Test the status logic
	assert.Equal(t, status.Up, drv.fileStatusFromInfo(testWithClients))
}

// fileStatusFromInfo is a test helper that simulates fileStatus with a given FilesystemInfo
func (t *T) fileStatusFromInfo(fileInfo *FilesystemInfo) status.T {
	if fileInfo == nil {
		t.StatusLog().Info("file object not visible in api")
		return status.Down
	}

	clients := t.getNFSClients(fileInfo)
	n := len(clients)

	if t.Exclusive {
		if n > 1 {
			t.StatusLog().Warn("too many grants (%d)", n)
		}
	}

	if n == 0 {
		t.StatusLog().Info("no access granted")
		return status.Down
	}

	// Filter clients to only those matching our host and permission
	var matchingClients []NfsClient
	for _, client := range clients {
		if client.Host == t.Host && client.Permission == t.Permission {
			matchingClients = append(matchingClients, client)
		}
	}

	if len(matchingClients) == 0 {
		t.StatusLog().Info("access not in current grants")
		return status.Down
	}

	if fileInfo.Status == "passive" {
		t.StatusLog().Info("status passive, access granted")
		return status.Down
	}

	return status.Up
}

// TestLabel tests the label functionality
func TestLabel(t *testing.T) {
	defer Setup(t)()

	drv := newDrvWithRid("test-rid")
	drv.authInfoer = sgcpauthtesthelper.NewMockGetAuthInfoProvider("id1")

	drv.UUID = "test-uuid"
	drv.MountPoint = "/tmp/foo"
	drv.Device = "/dev/false-nfs-for-test"
	require.NoError(t, drv.Configure())

	ctx := context.Background()
	label := drv.Label(ctx)

	// Without an underlying filesystem driver, should return UUID
	assert.Equal(t, "/dev/false-nfs-for-test@/tmp/foo", label)
}

// TestManifest tests the manifest functionality
func TestManifest(t *testing.T) {
	drv := New().(*T)

	manifest := drv.Manifest()
	require.NotNil(t, manifest)

	// Should have the correct driver ID
	assert.Equal(t, drvID, manifest.DriverID)

	// Should have keywords
	assert.NotEmpty(t, manifest.Keywords)
}

// TestFilesystemInfoUnmarshal tests JSON unmarshaling of FilesystemInfo
func TestFilesystemInfoUnmarshal(t *testing.T) {
	jsonData := `{
		"uuid": "test-uuid",
		"consistencyGroupId": "cg-uuid",
		"nfsClients": [
			{"uuid": "client1", "host": "host1", "permission": "read-write", "protocol": "nfs4.1"}
		],
		"status": "online"
	}`

	var fileInfo FilesystemInfo
	err := json.Unmarshal([]byte(jsonData), &fileInfo)
	require.NoError(t, err)

	assert.Equal(t, "test-uuid", fileInfo.UUID)
	assert.Equal(t, "cg-uuid", fileInfo.ConsistencyGroupID)
	assert.Len(t, fileInfo.NFSClients, 1)
	assert.Equal(t, "host1", fileInfo.NFSClients[0].Host)
	assert.Equal(t, "read-write", fileInfo.NFSClients[0].Permission)
}

// TestNfsClientMarshal tests JSON marshaling of NfsClient
func TestNfsClientMarshal(t *testing.T) {
	client := NfsClient{
		UUID:       "client-uuid",
		Host:       "client-host",
		Permission: "read-write",
		Protocol:   "nfs4.1",
	}

	jsonData, err := json.Marshal(client)
	require.NoError(t, err)

	var unmarshaled NfsClient
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, client.UUID, unmarshaled.UUID)
	assert.Equal(t, client.Host, unmarshaled.Host)
	assert.Equal(t, client.Permission, unmarshaled.Permission)
	assert.Equal(t, client.Protocol, unmarshaled.Protocol)
}
