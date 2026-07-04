//go:build !linux

package pg

// ApplyProc creates the cgroup, set caps, and add the specified process
func (c Config) ApplyProc(pid int) (bool, error) {
	return false, nil
}

func (c Config) Delete() (bool, error) {
	return false, nil
}
