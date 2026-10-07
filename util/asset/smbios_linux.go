//go:build linux

package asset

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync"

	"github.com/digitalocean/go-smbios/smbios"
)

type (
	// memoryDevice is a memory device structure of the SMBIOS, type 17,
	// as the DMTF SMBIOS specification (DSP0134) section 7.18 lays it out.
	//
	// The offsets of the specification count from the start of the
	// structure, its 4 bytes header included, and the formatted area the
	// decoder hands out starts after the header: the field at offset 0x10
	// is at index 0x0C of the formatted area.
	memoryDevice struct {
		*smbios.Structure
	}
)

var (
	smbiosOnce       sync.Once
	smbiosStructures []*smbios.Structure
	smbiosErr        error

	// memoryTypes are the names of the memory types, by their code
	// (DSP0134 7.18.2).
	memoryTypes = map[byte]string{
		0x01: "Other",
		0x02: "Unknown",
		0x03: "DRAM",
		0x04: "EDRAM",
		0x05: "VRAM",
		0x06: "SRAM",
		0x07: "RAM",
		0x08: "ROM",
		0x09: "FLASH",
		0x0A: "EEPROM",
		0x0B: "FEPROM",
		0x0C: "EPROM",
		0x0D: "CDRAM",
		0x0E: "3DRAM",
		0x0F: "SDRAM",
		0x10: "SGRAM",
		0x11: "RDRAM",
		0x12: "DDR",
		0x13: "DDR2",
		0x14: "DDR2 FB-DIMM",
		0x18: "DDR3",
		0x19: "FBD2",
		0x1A: "DDR4",
		0x1B: "LPDDR",
		0x1C: "LPDDR2",
		0x1D: "LPDDR3",
		0x1E: "LPDDR4",
		0x1F: "Logical non-volatile device",
		0x20: "HBM",
		0x21: "HBM2",
		0x22: "DDR5",
		0x23: "LPDDR5",
		0x24: "HBM3",
	}

	// typeDetails are the names of the bits of the memory type detail,
	// lowest bit first (DSP0134 7.18.3). Bit 0 is reserved.
	typeDetails = []string{
		"",
		"Other",
		"Unknown",
		"Fast-paged",
		"Static column",
		"Pseudo-static",
		"RAMBUS",
		"Synchronous",
		"CMOS",
		"EDO",
		"Window DRAM",
		"Cache DRAM",
		"Non-volatile",
		"Registered (Buffered)",
		"Unbuffered (Unregistered)",
		"LRDIMM",
	}
)

// readSMBIOS returns the structures of the SMBIOS of the node, read once.
func readSMBIOS() ([]*smbios.Structure, error) {
	smbiosOnce.Do(func() {
		rc, _, err := smbios.Stream()
		if err != nil {
			smbiosErr = err
			return
		}
		defer func() { _ = rc.Close() }()
		smbiosStructures, smbiosErr = smbios.NewDecoder(rc).Decode()
	})
	return smbiosStructures, smbiosErr
}

// memoryDevices returns the memory device structures of the SMBIOS.
func memoryDevices() ([]memoryDevice, error) {
	l, err := readSMBIOS()
	if err != nil {
		return nil, err
	}
	devs := make([]memoryDevice, 0)
	for _, s := range l {
		if s.Header.Type == 17 {
			devs = append(devs, memoryDevice{s})
		}
	}
	return devs, nil
}

// byteAt is the byte of the formatted area at the offset of the
// specification, 0 for a structure too short to have it, as one of an older
// version of the specification.
func (t memoryDevice) byteAt(offset int) byte {
	i := offset - 4
	if i < 0 || i >= len(t.Formatted) {
		return 0
	}
	return t.Formatted[i]
}

// wordAt is the little endian word of the formatted area at the offset of
// the specification, 0 for a structure too short to have it.
func (t memoryDevice) wordAt(offset int) uint16 {
	i := offset - 4
	if i < 0 || i+2 > len(t.Formatted) {
		return 0
	}
	return binary.LittleEndian.Uint16(t.Formatted[i : i+2])
}

// stringAt is the string the string number at the offset of the
// specification names, empty for none: number 0 says the structure has no
// such string, and number n is the n-th string after the formatted area.
func (t memoryDevice) stringAt(offset int) string {
	n := int(t.byteAt(offset))
	if n == 0 || n > len(t.Strings) {
		return ""
	}
	return strings.TrimSpace(t.Strings[n-1])
}

// Locator is the socket or board position of the device, as "DIMM 0".
func (t memoryDevice) Locator() string { return t.stringAt(0x10) }

// BankLocator is the bank of the device, as "Bank 0".
func (t memoryDevice) BankLocator() string { return t.stringAt(0x11) }

// Manufacturer is the manufacturer of the device.
func (t memoryDevice) Manufacturer() string { return t.stringAt(0x17) }

// PartNumber is the part number of the device.
func (t memoryDevice) PartNumber() string { return t.stringAt(0x1A) }

// MemoryType is the name of the memory type of the device, as "DDR4".
func (t memoryDevice) MemoryType() string {
	code := t.byteAt(0x12)
	if s, ok := memoryTypes[code]; ok {
		return s
	}
	return fmt.Sprintf("type 0x%02X", code)
}

// TypeDetail is the names of the bits of the type detail of the device set,
// as "Synchronous Registered (Buffered)".
func (t memoryDevice) TypeDetail() string {
	v := t.wordAt(0x13)
	l := make([]string, 0)
	for bit, name := range typeDetails {
		if name != "" && v&(1<<bit) != 0 {
			l = append(l, name)
		}
	}
	return strings.Join(l, " ")
}

// Speed is the maximum speed of the device, as "3200 MT/s", the extended
// speed of a device of 65535 MT/s or more, and "Unknown" for a speed the
// structure does not say.
func (t memoryDevice) Speed() string {
	speed := uint32(t.wordAt(0x15))
	if speed == 0xFFFF {
		i := 0x54 - 4
		if i+4 <= len(t.Formatted) {
			speed = binary.LittleEndian.Uint32(t.Formatted[i:i+4]) & 0x7FFFFFFF
		}
	}
	if speed == 0 {
		return "Unknown"
	}
	return fmt.Sprintf("%d MT/s", speed)
}
