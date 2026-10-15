//go:build linux

package asset

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
)

// cpuTopology is the count of the physical processors and of their cores,
// as /proc/cpuinfo describes them.
type cpuTopology struct {
	dies  int
	cores int
}

// readCPUTopology counts the processors and cores of /proc/cpuinfo.
//
// It counts them on a virtual machine too, where they are the ones the
// hypervisor shows, as v2 did: the sysinfo package leaves them at zero
// there.
func readCPUTopology() (cpuTopology, error) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return cpuTopology{}, err
	}
	defer f.Close()
	return parseCPUTopology(f)
}

// parseCPUTopology counts the distinct physical ids as the dies, and the
// distinct core ids of each as the cores. A cpuinfo without physical id, as
// a vmware guest, an arm or a s390 one shows, counts each processor as a
// die, and a cpuinfo without core id counts each die as a core.
func parseCPUTopology(r io.Reader) (cpuTopology, error) {
	var (
		physicalID      string
		processors      int
		processorsCount int
	)
	dies := make(map[string]bool)
	cores := make(map[string]bool)
	s := bufio.NewScanner(r)
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch k = strings.TrimSpace(k); {
		case k == "# processors":
			// s390 says the count, and names its processors
			// "processor 0", "processor 1", ...
			if n, err := strconv.Atoi(v); err == nil {
				processorsCount = n
			}
		case k == "processor":
			processors++
			physicalID = ""
		case k == "physical id":
			physicalID = v
			dies[v] = true
		case k == "core id":
			cores[physicalID+"/"+v] = true
		}
	}
	if err := s.Err(); err != nil {
		return cpuTopology{}, err
	}
	t := cpuTopology{dies: len(dies), cores: len(cores)}
	if processors == 0 {
		processors = processorsCount
	}
	if t.dies == 0 {
		t.dies = processors
	}
	if t.cores == 0 {
		t.cores = t.dies
	}
	return t, nil
}
