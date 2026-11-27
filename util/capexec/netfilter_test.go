package capexec

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/cgroupbpf"
)

type fakeNetFilterQuerier map[string]int

func (t fakeNetFilterQuerier) Effective(dir string, _ cgroupbpf.Direction) (int, error) {
	return t[dir], nil
}

func TestNetFilterRefusesACommandThatWouldRunUnfiltered(t *testing.T) {
	savedQuerier, savedDir := netFilterQuerier, netFilterDir
	t.Cleanup(func() { netFilterQuerier, netFilterDir = savedQuerier, savedDir })
	netFilterQuerier = fakeNetFilterQuerier{
		"/sys/fs/cgroup/a.slice/a.service": 1,
		"/sys/fs/cgroup/b.slice/b.service": 0,
	}
	yes, no := true, false
	in := func(dir string) {
		netFilterDir = func() (string, error) { return dir, nil }
	}

	in("/sys/fs/cgroup/b.slice/b.service")
	assert.NoError(t, T{}.checkNetFilter(), "not asked")
	assert.NoError(t, T{NetFilter: &no}.checkNetFilter(), "not asked")
	assert.EqualError(t, T{NetFilter: &yes}.checkNetFilter(),
		"--net-filter: no network filter is attached to /sys/fs/cgroup/b.slice/b.service for the ingress packets: the command is not run unfiltered")

	in("/sys/fs/cgroup/a.slice/a.service")
	assert.NoError(t, T{NetFilter: &yes}.checkNetFilter())

	netFilterDir = func() (string, error) { return "", errors.New("no cgroup") }
	assert.EqualError(t, T{NetFilter: &yes}.checkNetFilter(), "--net-filter: no cgroup")
}

func TestArgvPassesNetFilterOn(t *testing.T) {
	yes, no := true, false
	assert.Contains(t, T{NetFilter: &yes}.Argv(), "--net-filter")
	assert.NotContains(t, T{NetFilter: &no}.Argv(), "--net-filter")
	assert.NotContains(t, T{}.Argv(), "--net-filter")
}
