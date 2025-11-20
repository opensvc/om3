//go:build darwin

package resfsflag

func (t *T) baseDir() string {
	return t.VarDir()
}
