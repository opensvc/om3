// +build !linux

package md

const (
	mdadm string = "/bin/false"
)

func IsCapable() bool {
	return false
}
