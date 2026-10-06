package chknuma

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/check"
)

func TestResults(t *testing.T) {
	assert.Nil(t, results([]node{{name: "node0", cpus: 4, mem: 1000}}))
	assert.Equal(t, []check.Result{
		{Instance: "node0.mem.leveling", Value: 20, Unit: "%"},
		{Instance: "node1.mem.leveling", Value: 20, Unit: "%"},
	}, results([]node{{name: "node0", cpus: 4, mem: 1200}, {name: "node1", cpus: 4, mem: 800}}))
}

func TestMemTotal(t *testing.T) {
	assert.Equal(t, int64(16314424), memTotal([]byte("Node 0 MemTotal:       16314424 kB\nNode 0 MemFree:  1 kB\n")))
}
