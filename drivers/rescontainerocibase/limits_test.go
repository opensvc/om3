package rescontainerocibase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/util/ulimit"
)

// ulimitArgs returns the values of the --ulimit options of a container run,
// in the two forms the engines take, in their order.
func ulimitArgs(t *testing.T, limit ulimit.Config, runArgs ...string) []string {
	t.Helper()
	bt := &BT{
		T:       resource.T{ResourceID: &resourceid.T{Name: "container#1"}},
		Path:    naming.Path{Name: "foo", Kind: naming.KindSvc},
		Limit:   limit,
		RunArgs: runArgs,
	}
	require.NoError(t, bt.Configure())
	ea := ExecutorArg{BT: bt}
	base, err := ea.RunArgsBase(context.Background())
	require.NoError(t, err)
	l := make([]string, 0)
	a := base.Get()
	for i, s := range a {
		if s == "--ulimit" && i+1 < len(a) {
			l = append(l, a[i+1])
		} else if v, ok := strings.CutPrefix(s, "--ulimit="); ok {
			l = append(l, v)
		}
	}
	return l
}

func ptr[T any](v T) *T { return &v }

// A limit_* keyword is the ulimit of the name the engine knows it by, its
// soft and hard limits set to the value, as om exec sets the ones of an app.
// A container with none is given no --ulimit, and runs with the defaults of
// the engine.
func TestTheLimitKeywordsAreTheUlimitsOfTheContainer(t *testing.T) {
	assert.Empty(t, ulimitArgs(t, ulimit.Config{}), "no keyword, no option")

	assert.Equal(t, []string{"nofile=1024:1024"}, ulimitArgs(t, ulimit.Config{NoFile: ptr(int64(1024))}))

	assert.Equal(t,
		[]string{
			"cpu=90:90",
			"fsize=1048576:1048576",
			"data=2097152:2097152",
			"stack=8388608:8388608",
			"core=0:0",
			"nofile=4096:4096",
			"as=4294967296:4294967296",
			"memlock=65536:65536",
			"nproc=512:512",
			"rss=1073741824:1073741824",
		},
		ulimitArgs(t, ulimit.Config{
			CPU:     ptr(90 * time.Second),
			FSize:   ptr(int64(1 << 20)),
			Data:    ptr(int64(2 << 20)),
			Stack:   ptr(int64(8 << 20)),
			Core:    ptr(int64(0)),
			NoFile:  ptr(int64(4096)),
			AS:      ptr(int64(4 << 30)),
			MemLock: ptr(int64(64 << 10)),
			NProc:   ptr(int64(512)),
			RSS:     ptr(int64(1 << 30)),
		}))
}

// limit_vmem and limit_as are one limit of the kernel, the address space:
// either sets it, and the larger does when both are set.
func TestLimitVmemAndLimitAsAreTheAddressSpace(t *testing.T) {
	assert.Equal(t, []string{"as=1024:1024"}, ulimitArgs(t, ulimit.Config{VMem: ptr(int64(1024))}))
	assert.Equal(t, []string{"as=1024:1024"}, ulimitArgs(t, ulimit.Config{AS: ptr(int64(1024))}))
	assert.Equal(t, []string{"as=2048:2048"}, ulimitArgs(t, ulimit.Config{AS: ptr(int64(1024)), VMem: ptr(int64(2048))}))
	assert.Equal(t, []string{"as=2048:2048"}, ulimitArgs(t, ulimit.Config{AS: ptr(int64(2048)), VMem: ptr(int64(1024))}))
}

// A limit a keyword sets is dropped from run_args, in either form of the
// option, as --hostname is under the hostname keyword: the engine would be
// given the limit twice. The limits no keyword sets are left to run_args.
func TestALimitKeywordOverridesTheUlimitOfRunArgs(t *testing.T) {
	assert.ElementsMatch(t,
		[]string{"nofile=1024:1024", "nproc=100:200", "core=0"},
		ulimitArgs(t, ulimit.Config{NoFile: ptr(int64(1024))},
			"--ulimit", "nofile=65536:65536",
			"--ulimit=nofile=4096",
			"--ulimit", "nproc=100:200",
			"--ulimit=core=0",
		))

	assert.ElementsMatch(t,
		[]string{"nofile=65536:65536"},
		ulimitArgs(t, ulimit.Config{}, "--ulimit", "nofile=65536:65536"),
		"with no keyword, run_args decides")
}
