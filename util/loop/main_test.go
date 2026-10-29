//go:build linux

package loop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// TestDeleteWaitsForAutoclear checks a device listed for a while after its
// deletion, as one still open is until its last close, is seen gone once the
// kernel detaches it.
func TestDeleteWaitsForAutoclear(t *testing.T) {
	td := t.TempDir()
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": td})
	defer rawconfig.Load(map[string]string{})

	// The fake losetup lists the device at its first two listings after the
	// delete, and no more after.
	bin := filepath.Join(td, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	counter := filepath.Join(td, "listings")
	script := `#!/bin/sh
case "$1" in
-d) exit 0 ;;
esac
n=$(cat ` + counter + ` 2>/dev/null || echo 0)
n=$((n+1))
echo $n >` + counter + `
echo "NAME       BACK-FILE"
if [ $n -le 2 ]; then
	echo "/dev/loop3 /var/tmp/test.img"
fi
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "losetup"), []byte(script), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "udevadm"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	require.NoError(t, New().Delete(context.Background(), "/dev/loop3"))
}
