package pg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The values refused are the ones the apply refuses, or the kernel does, so
// a configuration holding one is refused when written rather than failing
// the next start or cap on every node.
func TestValidValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid func(string) error
		ok    []string
		bad   []string
	}{
		{"cpus", ValidCPUs, []string{"0", "0-2", "0,1,2", "0-1,4,6-7", DefaultValue}, []string{"a", "0-", "-1", "2-1", "0,,1", "0 1"}},
		{"cpu quota", ValidCPUQuota, []string{"50%", "50", "50%@all", "10%@2", "50%@", "300%", DefaultValue}, []string{"50%@x", "x%", "-5%", "50%@0", "50%@-1", "50%@2@3", "%"}},
		{"size", ValidSize, []string{"512m", "1g", "1024", DefaultValue}, []string{"x", "1q", "-1"}},
		{"pids max", ValidPidsMax, []string{"1", "4096", DefaultValue}, []string{"0", "-1", "max", "1k"}},
		{"blkio weight", ValidBlkioWeight, []string{"1", "100", "10000", DefaultValue}, []string{"0", "10001", "x"}},
		{"mem swappiness", ValidMemSwappiness, []string{"0", "60", "100"}, []string{"101", "-1", "x", DefaultValue}},
		{"mem oom control", ValidMemOOMControl, []string{"0", "1", "true"}, []string{"2", "x", DefaultValue}},
	} {
		for _, s := range tc.ok {
			assert.NoErrorf(t, tc.valid(s), "%s %q", tc.name, s)
		}
		for _, s := range tc.bad {
			assert.Errorf(t, tc.valid(s), "%s %q", tc.name, s)
		}
	}
}
