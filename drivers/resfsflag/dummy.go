//go:build !linux && !solaris && !darwin

package resfsflag

func (t *T) baseDir() string {
	panic("not implemented")
	return ""
}

// nodeBaseDir returns no base directory.
func nodeBaseDir() string {
	return ""
}
