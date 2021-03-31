// +build !linux,!solaris

package resfsflag

func (t T) baseDir() string {
	panic("not implemented")
	return ""
}
