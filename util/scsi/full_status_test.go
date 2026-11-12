package scsi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The full status of a lun registered by two paths of a node, the first
// holding the reservation, as sg_persist and mpathpersist write it.
const (
	sgFullStatus = `  PR generation=0x2
    Key=0xdead01
      All target ports bit clear
      Relative port address: 0xa
      << Reservation holder >>
      scope: LU_SCOPE,  type: Write Exclusive, registrants only
      Transport Id of initiator:
        iSCSI world wide unique port id: iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000002
    Key=0xdead01
      All target ports bit clear
      Relative port address: 0xb
      not reservation holder
      Transport Id of initiator:
        iSCSI world wide unique port id: iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000001
`
	mpathFullStatus = `  PR generation=0x2
   Key = 0xdead01
      All target ports bit clear
      Relative port address: 0xa
      << Reservation holder >>
      scope: LU_SCOPE,  type: Write Exclusive, registrants only
   iSCSI world wide unique port id: iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000002
   Key = 0xdead01
      All target ports bit clear
      Relative port address: 0xb
      not reservation holder
   iSCSI world wide unique port id: iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000001
`
)

func TestParseFullStatus(t *testing.T) {
	want := []FullStatusEntry{
		{Key: "0x0000000000dead01", RelativeTargetPort: 0xa, Holder: true, TransportID: "iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000002"},
		{Key: "0x0000000000dead01", RelativeTargetPort: 0xb, TransportID: "iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000001"},
	}
	assert.Equal(t, want, parseFullStatus([]byte(sgFullStatus)), "sg_persist")
	assert.Equal(t, want, parseFullStatus([]byte(mpathFullStatus)), "mpathpersist")
	assert.Empty(t, parseFullStatus([]byte("  PR generation=0x2\n  No full status descriptors\n")))
}

// open-iscsi gives a session the id 00 02 3d 00 then its session number:
// session2 registered as 0x00023d000002.
func TestOpenISCSIISID(t *testing.T) {
	assert.Equal(t, "0x00023d000002", openISCSIISID(2))
	assert.True(t, sameISCSIPort("IQN.2009-11.com.opensvc.srv:n1,i,0x23d000002", "iqn.2009-11.com.opensvc.srv:n1,i,"+openISCSIISID(2)))
	assert.False(t, sameISCSIPort("iqn.x:n1,i,0x00023d000001", "iqn.x:n1,i,0x00023d000002"))
	assert.False(t, sameISCSIPort("iqn.x:n1,i,0x00023d000001", "iqn.x:n2,i,0x00023d000001"))
}

// A registration is of a current path when its initiator port and its target
// port are the ones of a current path: the session id alone is not, the same
// session reaching the lun by another target port.
func TestIsCurrent(t *testing.T) {
	entries := parseFullStatus([]byte(sgFullStatus))
	paths := []nexus{{TransportID: "iqn.2009-11.com.opensvc.srv:dev2n1.storage.initiator,i,0x00023d000002", RelativeTargetPort: 0xa}}
	assert.True(t, entries[0].isCurrent(paths))
	assert.False(t, entries[1].isCurrent(paths), "another session")
	paths[0].RelativeTargetPort = 0xb
	assert.False(t, entries[0].isCurrent(paths), "another target port")
}
