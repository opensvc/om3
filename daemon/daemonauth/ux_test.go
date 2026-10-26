package daemonauth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

type listenAddr string

func (t listenAddr) ListenAddr(context.Context) string {
	return string(t)
}

func TestUXStrategy(t *testing.T) {
	t.Run("the root socket is root", func(t *testing.T) {
		info, err := uxStrategy{getter: listenAddr(daemonenv.HTTPUnixFile())}.Authenticate(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, []string{rbac.GrantRoot.String()}, info.Grants)
	})
	t.Run("another unix socket is not", func(t *testing.T) {
		other := filepath.Join(filepath.Dir(filepath.Dir(daemonenv.HTTPUnixFile())), "uxuser", "http.sock")
		_, err := uxStrategy{getter: listenAddr(other)}.Authenticate(context.Background(), nil)
		assert.Error(t, err)
	})
	t.Run("an inet address is not", func(t *testing.T) {
		_, err := uxStrategy{getter: listenAddr("127.0.0.1:1215")}.Authenticate(context.Background(), nil)
		assert.Error(t, err)
	})
}
