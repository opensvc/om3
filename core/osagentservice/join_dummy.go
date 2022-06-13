//go:build !linux

package osagentservice

// Join will add current process to the system opensvc agent
func Join() error {
	return nil
}
