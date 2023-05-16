//go:build !linux

package nmon

// osBootedWithOpensvcFreeze returns true if os has been booted with opensvc frozen
func osBootedWithOpensvcFreeze() bool {
	// TODO implement for Solaris using 'prtconf -v | ggrep -A 1 bootargs(' that contains -x
	return false
}
