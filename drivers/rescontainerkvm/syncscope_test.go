package rescontainerkvm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The files to sync are the ones of the container the node evaluates: the
// configuration of vm1 is not the one of the vm2 the same resource runs on
// another node.
func TestSyncScopeIsTheContainerName(t *testing.T) {
	assert.Equal(t, "vm2", (&T{Name: "vm2"}).SyncScope())
	assert.Equal(t, "/etc/libvirt/qemu/vm2.xml", (&T{Name: "vm2"}).configFile())
}
