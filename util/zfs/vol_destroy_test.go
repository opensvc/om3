package zfs

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// fakeZfs puts on the PATH a zfs answering the destroys from the lines of
// answers, one a run, as "busy", "gone", "fail" or "ok", and returns the
// file counting the runs.
func fakeZfs(t *testing.T, answers ...string) string {
	t.Helper()
	dir := t.TempDir()
	count := filepath.Join(dir, "count")
	script := "#!/bin/sh\nn=$(cat " + count + " 2>/dev/null || echo 0)\nn=$((n+1))\necho $n >" + count + "\ncase $n in\n"
	for i, answer := range answers {
		script += strconv.Itoa(i+1) + ") "
		switch answer {
		case "busy":
			script += "echo \"cannot destroy '$3': dataset is busy\" >&2; exit 1;;\n"
		case "gone":
			script += "echo \"cannot open '$3': dataset does not exist\" >&2; exit 1;;\n"
		case "fail":
			script += "echo \"cannot destroy '$3': permission denied\" >&2; exit 1;;\n"
		default:
			script += "exit 0;;\n"
		}
	}
	script += "esac\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zfs"), []byte(script), 0755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	was := volDestroyBusyDelay
	volDestroyBusyDelay = 0
	t.Cleanup(func() { volDestroyBusyDelay = was })
	return count
}

func runs(t *testing.T, count string) int {
	t.Helper()
	b, err := os.ReadFile(count)
	require.NoError(t, err)
	n, err := strconv.Atoi(string(b[:len(b)-1]))
	require.NoError(t, err)
	return n
}

func TestVolDestroy(t *testing.T) {
	vol := Vol{Name: "tank/v1", Log: plog.NewDefaultLogger()}

	count := fakeZfs(t, "ok")
	assert.NoError(t, vol.Destroy(VolDestroyWithForce()))
	assert.Equal(t, 1, runs(t, count))

	count = fakeZfs(t, "fail")
	assert.Error(t, vol.Destroy(VolDestroyWithBusyRetries(3)), "a destroy failing is an error")
	assert.Equal(t, 1, runs(t, count), "and not tried again")

	count = fakeZfs(t, "busy", "busy", "ok")
	assert.NoError(t, vol.Destroy(VolDestroyWithBusyRetries(3)), "a busy volume is destroyed once released")
	assert.Equal(t, 3, runs(t, count))

	count = fakeZfs(t, "busy", "busy", "busy")
	assert.Error(t, vol.Destroy(VolDestroyWithBusyRetries(2)), "a volume busy past the retries is not destroyed")
	assert.Equal(t, 3, runs(t, count))

	count = fakeZfs(t, "busy")
	assert.Error(t, vol.Destroy(), "with no retry, busy is an error")
	assert.Equal(t, 1, runs(t, count))

	count = fakeZfs(t, "gone")
	assert.NoError(t, vol.Destroy(), "a volume already gone is destroyed")
	assert.Equal(t, 1, runs(t, count))
}
