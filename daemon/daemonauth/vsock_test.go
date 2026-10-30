package daemonauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonctx"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

func TestVsockStrategy(t *testing.T) {
	s := vsockStrategy{}

	info, err := s.Authenticate(daemonctx.WithVsockPeer(context.Background(), vsockHostCID, 1023), nil)
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
		_, err := s.Authenticate(tc.ctx, nil)
		assert.Error(t, err, tc.name)
	}
}
