package daemonauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/vsockauth"
	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

const vsockTestGuest = "de857c8e-280f-49ba-a9e5-b488688f2c8e"

// vsockTestStrategy returns a strategy reading the firmware of a guest told
// the key.
func vsockTestStrategy(t *testing.T, key *rsa.PrivateKey) vsockStrategy {
	t.Helper()
	dir := t.TempDir()
	s := vsockStrategy{dmiEntries: filepath.Join(dir, "entries"), productUUID: filepath.Join(dir, "uuid")}
	require.NoError(t, os.WriteFile(s.productUUID, []byte(strings.ToUpper(vsockTestGuest)+"\n"), 0o400))
	require.NoError(t, os.MkdirAll(filepath.Join(s.dmiEntries, "11-0"), 0o755))
	oem, err := vsockauth.OEMString(&key.PublicKey)
	require.NoError(t, err)
	table := append([]byte{11, 5, 0, 1, 1}, oem...)
	require.NoError(t, os.WriteFile(filepath.Join(s.dmiEntries, "11-0", "raw"), append(table, 0, 0), 0o400))
	return s
}

func vsockRequest(t *testing.T, token string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

func TestVsockStrategy(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	s := vsockTestStrategy(t, key)
	token, err := vsockauth.NewToken(key, vsockTestGuest)
	require.NoError(t, err)
	hypervisor := daemonctx.WithVsockPeer(context.Background(), vsockHostCID, 1023)

	info, err := s.Authenticate(hypervisor, vsockRequest(t, token))
	require.NoError(t, err)
	assert.Equal(t, StrategyVsock, info.Strategy)
	assert.Equal(t, []string{rbac.GrantRoot.String()}, info.Grants)

	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"not a vsock client", context.Background()},
		{"a source port any user can bind", daemonctx.WithVsockPeer(context.Background(), vsockHostCID, 1024)},
		{"a guest", daemonctx.WithVsockPeer(context.Background(), 3, 1023)},
		{"the loopback", daemonctx.WithVsockPeer(context.Background(), 1, 1023)},
	} {
		_, err := s.Authenticate(tc.ctx, vsockRequest(t, token))
		assert.Error(t, err, tc.name)
	}

	// A process of the hypervisor binding a reserved port, as
	// CAP_NET_BIND_SERVICE allows, reads neither key of the cluster.
	_, err = s.Authenticate(hypervisor, vsockRequest(t, ""))
	assert.Error(t, err, "a reserved port alone")

	forged, err := vsockauth.NewToken(other, vsockTestGuest)
	require.NoError(t, err)
	_, err = s.Authenticate(hypervisor, vsockRequest(t, forged))
	assert.Error(t, err, "another key")

	otherGuest, err := vsockauth.NewToken(key, "20b1a61c-605b-4a9d-9a57-a75f5e0d3db0")
	require.NoError(t, err)
	_, err = s.Authenticate(hypervisor, vsockRequest(t, otherGuest))
	assert.Error(t, err, "a token another guest received")

	s.dmiEntries = t.TempDir()
	_, err = s.Authenticate(hypervisor, vsockRequest(t, token))
	assert.ErrorIs(t, err, vsockauth.ErrNoKey, "a guest not told the key grants nothing")
}
