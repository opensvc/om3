//go:build !linux

package systemd

// HasSystemd return true if systemd is detected on current os
// It always return false on non linux systems
func HasSystemd() bool {
	return false
}

func Escape(s string) string {
	return " "
}
