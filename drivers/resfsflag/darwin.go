//go:build darwin

package resfsflag

func (t *T) baseDir() string {
	return t.VarDir()
}

// nodeBaseDir returns no base directory: the flag files are in the var
// directory of each object.
func nodeBaseDir() string {
	return ""
}
