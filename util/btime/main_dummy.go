//go:build !linux && !darwin

package btime

import "fmt"

func bootTime() (uint64, error) {
	return 0, fmt.Errorf("boot time is not implemented on this platform")
}
