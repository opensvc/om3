//go:build linux

package asset

import (
	"encoding/binary"
	"testing"

	"github.com/digitalocean/go-smbios/smbios"
	"github.com/stretchr/testify/assert"
)

// newMemoryDevice returns a memory device structure of the SMBIOS 3.3 size,
// with the fields set at their offsets of the specification.
func newMemoryDevice(set func(b []byte), strs ...string) memoryDevice {
	b := make([]byte, 0x5C-4)
	set(b)
	return memoryDevice{&smbios.Structure{
		Header:    smbios.Header{Type: 17, Length: 0x5C},
		Formatted: b,
		Strings:   strs,
	}}
}

// at is the index in the formatted area of an offset of the specification.
func at(offset int) int { return offset - 4 }

func TestMemoryDeviceStringsAreReadByTheirNumber(t *testing.T) {
	dev := newMemoryDevice(func(b []byte) {
		b[at(0x10)] = 1 // locator
		b[at(0x11)] = 0 // no bank locator
		b[at(0x17)] = 2 // manufacturer
		b[at(0x1A)] = 3 // part number
	}, "DIMM 0", "QEMU", "PN-1 ")
	assert.Equal(t, "DIMM 0", dev.Locator())
	assert.Equal(t, "", dev.BankLocator(), "number 0 is no string")
	assert.Equal(t, "QEMU", dev.Manufacturer())
	assert.Equal(t, "PN-1", dev.PartNumber())
	assert.Equal(t, "DIMM 0", joinFields(dev.Locator(), dev.BankLocator()))
}

func TestMemoryDeviceTypeDetailNamesTheBitsLowestFirst(t *testing.T) {
	dev := newMemoryDevice(func(b []byte) {
		binary.LittleEndian.PutUint16(b[at(0x13):], 1<<1)
	})
	assert.Equal(t, "Other", dev.TypeDetail())
	dev = newMemoryDevice(func(b []byte) {
		binary.LittleEndian.PutUint16(b[at(0x13):], 1<<7|1<<13)
	})
	assert.Equal(t, "Synchronous Registered (Buffered)", dev.TypeDetail())
}

func TestMemoryDeviceTypeAndSpeed(t *testing.T) {
	dev := newMemoryDevice(func(b []byte) {
		b[at(0x12)] = 0x1A
		binary.LittleEndian.PutUint16(b[at(0x15):], 3200)
	})
	assert.Equal(t, "DDR4", dev.MemoryType())
	assert.Equal(t, "3200 MT/s", dev.Speed())

	dev = newMemoryDevice(func(b []byte) {
		b[at(0x12)] = 0x99
	})
	assert.Equal(t, "type 0x99", dev.MemoryType(), "a code newer than the table does not panic")
	assert.Equal(t, "Unknown", dev.Speed(), "speed 0 is unknown")

	dev = newMemoryDevice(func(b []byte) {
		binary.LittleEndian.PutUint16(b[at(0x15):], 0xFFFF)
		binary.LittleEndian.PutUint32(b[at(0x54):], 70000)
	})
	assert.Equal(t, "70000 MT/s", dev.Speed(), "the extended speed")
}

func TestMemoryDeviceOfAnOlderSpecificationIsShorter(t *testing.T) {
	dev := memoryDevice{&smbios.Structure{
		Header:    smbios.Header{Type: 17, Length: 0x15},
		Formatted: make([]byte, 0x15-4),
	}}
	assert.Equal(t, "", dev.Manufacturer(), "a field past the structure is not set")
	assert.Equal(t, "Unknown", dev.Speed())
}
