// +build linux

package main

import (
	"path/filepath"
)

func (t T) baseDir() string {
	return filepath.FromSlash("/tmp/opensvc")
}
