//go:build solaris

package resiphost

func (t *T) arpGratuitous() error {
	return nil
}

func (t *T) neighborAdvertise(dev string) error {
	return nil
}
