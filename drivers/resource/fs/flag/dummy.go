// +build !linux,!solaris

package main

func (t T) baseDir() string {
	panic("not implemented")
	return ""
}
