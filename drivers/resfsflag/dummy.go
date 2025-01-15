//go:build !linux && !solaris && !darwin

package resfsflag

func (t *T) baseDir() string {
	panic("not implemented")
	return ""
}
