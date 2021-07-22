// +build !linux

package loop

func IsCapable() bool {
	return false
}
