//go:build !linux

package resiphost

func (t *T) arpGratuitous() error {
	return nil
}
