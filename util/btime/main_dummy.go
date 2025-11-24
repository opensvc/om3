//go:build !linux && !darwin

package btime

func bootTime() (uint64, error) {
	return 0, nil
}
