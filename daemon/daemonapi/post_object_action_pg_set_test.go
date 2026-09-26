package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
)


func TestAPGSetIsTheKeywordsOfItsSections(t *testing.T) {
	l, err := pgSetOps(map[string]api.PGCaps{
		"container#1": {CpuQuota: ptr("80%"), MemLimit: ptr("512m")},
		"DEFAULT":     {PidsMax: ptr("default")},
		"subset#web":  {MemHigh: ptr("1g")},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"DEFAULT.pg_pids_max=default",
		"container#1.pg_cpu_quota=80%",
		"container#1.pg_mem_limit=512m",
		"subset#web.pg_mem_high=1g",
	}, l)
}

// Only the sections holding caps are written, and a value carries no other
// keyword in a second line.
func TestAPGSetRefusesWhatIsNotACap(t *testing.T) {
	for name, caps := range map[string]map[string]api.PGCaps{
		"nothing":         {},
		"no cap":          {"container#1": {}},
		"not a section":   {"env": {CpuQuota: ptr("50%")}},
		"a key":           {"container#1.pg_cpus": {CpuQuota: ptr("50%")}},
		"a second line":   {"container#1": {CpuQuota: ptr("50%\n[app#1]\nstart = /bin/sh")}},
		"an empty subset": {"subset#": {CpuQuota: ptr("50%")}},
	} {
		_, err := pgSetOps(caps)
		assert.Error(t, err, name)
	}
}
