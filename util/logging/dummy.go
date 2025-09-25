//go:build !linux

package logging

func journalEnabled() bool {
	return false
}
