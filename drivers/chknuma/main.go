// Package chknuma is the numa check driver: how far the memory of each numa
// node is from its share of the memory, in percent, its share being the part
// of the memory its cpus would have with the memory spread evenly.
package chknuma

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "numa"
	// DriverName is the name of check driver.
	DriverName = "sysfs"
)

type (
	checker struct{}

	node struct {
		name string
		cpus int
		mem  int64
	}
)

var nodesDir = "/sys/devices/system/node"

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	for _, r := range results(nodes()) {
		r.DriverGroup = DriverGroup
		r.DriverName = DriverName
		rs.Push(r)
	}
	return rs, nil
}

func nodes() []node {
	dirs, _ := filepath.Glob(filepath.Join(nodesDir, "node[0-9]*"))
	var l []node
	for _, dir := range dirs {
		cpus, _ := filepath.Glob(filepath.Join(dir, "cpu[0-9]*"))
		b, err := os.ReadFile(filepath.Join(dir, "meminfo"))
		if err != nil {
			continue
		}
		l = append(l, node{name: filepath.Base(dir), cpus: len(cpus), mem: memTotal(b)})
	}
	return l
}

// memTotal returns the kB of the MemTotal line of a node meminfo.
func memTotal(b []byte) int64 {
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, "MemTotal") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		v, _ := strconv.ParseInt(fields[len(fields)-2], 10, 64)
		return v
	}
	return 0
}

// results are the deviations of the nodes, none on a single node system.
func results(nodes []node) []check.Result {
	if len(nodes) < 2 {
		return nil
	}
	var (
		mem  int64
		cpus int
	)
	for _, n := range nodes {
		mem += n.mem
		cpus += n.cpus
	}
	if cpus == 0 {
		return nil
	}
	var l []check.Result
	for _, n := range nodes {
		target := float64(mem) / float64(cpus) * float64(n.cpus)
		if target == 0 {
			continue
		}
		deviation := math.Abs(math.Floor(100 * (float64(n.mem) - target) / target))
		l = append(l, check.Result{Instance: n.name + ".mem.leveling", Value: int64(deviation), Unit: "%"})
	}
	return l
}
