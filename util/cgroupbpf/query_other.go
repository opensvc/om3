//go:build !linux

package cgroupbpf

import "errors"

// Kernel is the querier asking the kernel, which only a linux one answers.
type Kernel struct{}

// Effective implements Querier.
func (Kernel) Effective(dir string, direction Direction) (int, error) {
	return 0, errors.New("the programs of a cgroup are queried on linux only")
}

// Supported says the kernel answers the query.
func Supported() bool {
	return false
}
