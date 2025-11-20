//go:build !linux

package udevadm

func Settle() {
}

func Properties(dev string) (map[string]string, error) {
	return map[string]string{}, nil
}
