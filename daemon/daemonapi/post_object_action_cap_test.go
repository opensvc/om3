package daemonapi

import (
	"testing"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestACapIsTheSetOfPGKeywords(t *testing.T) {
	l, err := capOps(naming.KindSvc, []string{
		"pg_cpus=0-1",
		"container#1.pg_cpu_quota=80%",
		"subset#web.pg_mem_high=3g",
		"DEFAULT.pg_mem_limit@n1=4g",
		"app#1.pg_pids_max=default",
	})
	require.NoError(t, err)
	require.Len(t, l, 5)
	assert.Equal(t, "DEFAULT", l[0].Key.Section)
	assert.Equal(t, "pg_cpus", l[0].Key.Option)
	assert.Equal(t, "container#1", l[1].Key.Section)
	assert.Equal(t, "pg_mem_limit@n1", l[3].Key.Option)
	assert.Equal(t, "default", l[4].Value)

	l, err = capOps(naming.KindSvc, nil)
	require.NoError(t, err)
	assert.Empty(t, l, "no cap is the caps the configuration holds, applied again")
}

// A cap opens nothing a configuration update does not: another keyword, a
// section holding no caps and a list operator are refused.
func TestACapRefusesWhatIsNotACap(t *testing.T) {
	for _, s := range []string{
		"env.foo=bar",
		"app#1.start=/bin/sh",
		"pg_cpus+=2",
		"env.pg_cpus=0",
		"pg_cpus",
		"container#1.pg_cpu_quota=50%\n[app#1]",
		"pg_bogus=1",
		"pg_mem_limt@n1=4g",
	} {
		_, err := capOps(naming.KindSvc, []string{s})
		assert.Error(t, err, s)
	}
}

// A mistyped cap is refused with the caps there are, rather than written to
// cap nothing.
func TestACapOfAKeywordTheKindDoesNotHaveIsRefused(t *testing.T) {
	_, err := capOps(naming.KindSvc, []string{"pg_mem_limt=4g"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "pg_mem_limt is not a cap of a svc")
	assert.ErrorContains(t, err, "pg_mem_limit")
}
