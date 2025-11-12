//go:build !linux

package drbd

const (
	drbdadm   string = "/bin/false"
	drbdsetup string = "/bin/false"
)

func IsCapable() bool {
	return false
}
