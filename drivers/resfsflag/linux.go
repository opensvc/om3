//go:build linux

package resfsflag

func (t *T) baseDir() string {
	return tmpBaseDir()
}
