//go:build linux

package resipcni

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cni configuration of the network not read is the ConfigError of the
// resource, which a change fixes, when the namespace of the object is not
// running: nothing holds the address. Read, there is none.
func TestConfigError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	r := &T{CNIConfig: dir, Network: "gone", ObjectID: uuid.New()}
	assert.ErrorIs(t, r.ConfigError(ctx), os.ErrNotExist, "the network removed, its configuration gone")

	r.Network = "../gone"
	assert.ErrorContains(t, r.ConfigError(ctx), "a network is named, not a path")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "backend.conf"), []byte(`{"type": "bridge"}`), 0o644))
	r.Network = "backend"
	assert.NoError(t, r.ConfigError(ctx))
}
