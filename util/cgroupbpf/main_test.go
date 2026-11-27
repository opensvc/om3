package cgroupbpf

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeQuerier answers the count of programs of each direction of a cgroup
// directory from a table, and an error for a directory out of it.
type fakeQuerier map[string][2]int

func (t fakeQuerier) Effective(dir string, direction Direction) (int, error) {
	counts, ok := t[dir]
	if !ok {
		return 0, errors.New("operation not permitted")
	}
	return counts[direction], nil
}

func TestVerify(t *testing.T) {
	q := fakeQuerier{
		"/slice":                {0, 0},
		"/slice/unit.service":   {1, 1},
		"/slice/plain":          {0, 0},
		"/slice/egress.service": {0, 1},
		"/node":                 {2, 2},
		"/node/slice":           {2, 2},
		"/node/unit.service":    {3, 3},
	}
	assert.NoError(t, Verify(q, "/slice/unit.service", "/slice"))
	assert.NoError(t, Verify(q, "/node/unit.service", "/node"), "the programs of the unit count over the ones attached above")

	err := Verify(q, "/slice/plain", "/slice")
	assert.ErrorIs(t, err, ErrNotAttached)
	assert.EqualError(t, err, "no network filter is attached to /slice/plain for the ingress packets")

	err = Verify(q, "/slice/egress.service", "/slice")
	assert.ErrorIs(t, err, ErrNotAttached, "both directions are filtered, or none is")

	err = Verify(q, "/node/slice", "/node")
	assert.ErrorIs(t, err, ErrNotAttached, "the programs attached above are not the filter of the unit")

	err = Verify(q, "/elsewhere", "/slice")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotAttached, "a query the kernel refuses is not a filter known missing")
	assert.EqualError(t, err, "query the ingress programs of /elsewhere: operation not permitted")
}

func TestParseDir(t *testing.T) {
	saved := mount
	t.Cleanup(func() { mount = saved })
	mount = "/sys/fs/cgroup"
	read := func(content string) (string, error) {
		p := filepath.Join(t.TempDir(), "cgroup")
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		f, err := os.Open(p)
		require.NoError(t, err)
		defer func() { _ = f.Close() }()
		return parseDir(f)
	}
	dir, err := read("0::/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-app.1.slice/opensvc-svc.web-app.1.service\n")
	require.NoError(t, err)
	assert.Equal(t, "/sys/fs/cgroup/opensvc.slice/opensvc-svc.web.slice/opensvc-svc.web-app.1.slice/opensvc-svc.web-app.1.service", dir)

	dir, err = read("12:cpu,cpuacct:/a\n1:name=systemd:/b\n0::/c\n")
	require.NoError(t, err)
	assert.Equal(t, "/sys/fs/cgroup/c", dir, "the line of the unified hierarchy of a hybrid node")

	_, err = read("12:cpu,cpuacct:/a\n")
	assert.Error(t, err, "a node of the v1 hierarchy alone")
}
