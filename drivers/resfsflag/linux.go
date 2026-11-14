//go:build linux

package resfsflag

func (t *T) baseDir() string {
	return tmpBaseDir()
}

// nodeBaseDir returns the base directory of the flag files of the node.
func nodeBaseDir() string {
	return tmpBaseDir()
}
