// +build linux

package poolshm

func (t T) path() string {
	return "/dev/shm"
}
