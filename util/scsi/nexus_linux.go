//go:build linux

package scsi

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/device"
)

var (
	regexpISCSISession = regexp.MustCompile(`/session([0-9]+)/`)

	// regexpBlockName is a kernel block device name, as sda.
	regexpBlockName = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

	// sysRoot is the root of the sysfs paths, set by the tests.
	sysRoot = "/sys"
)

// currentNexus returns the current paths of the node to the device, as
// their nexus: the transport id of the initiator port, from the iscsi
// session of the path, and the relative target port, from the device
// identification page of the path. It returns the count of the paths it can
// not tell the nexus of too, as a fibre channel or an unreadable one: each is
// skipped, rather than the device, whose iscsi paths still tell.
func currentNexus(dev device.T) ([]nexus, int, error) {
	paths, err := dev.SCSIPaths()
	if err != nil {
		return nil, 0, err
	}
	l := make([]nexus, 0, len(paths))
	unknown := 0
	for _, p := range paths {
		n, err := pathNexus(p.Path())
		if err != nil {
			unknown++
			continue
		}
		l = append(l, n)
	}
	if len(l) == 0 {
		return nil, unknown, fmt.Errorf("%w: none of the %d paths of %s", ErrNoNexus, len(paths), dev)
	}
	return l, unknown, nil
}

// pathNexus returns the nexus of the scsi disk at path, as /dev/sda or a
// /dev/disk/by-id link to it.
func pathNexus(path string) (nexus, error) {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	name := filepath.Base(path)
	if !regexpBlockName.MatchString(name) {
		return nexus{}, fmt.Errorf("%w: %s is not a block device name", ErrNoNexus, name)
	}
	devicePath, err := filepath.EvalSymlinks(filepath.Join(sysRoot, "block", name, "device"))
	if err != nil {
		return nexus{}, err
	}
	m := regexpISCSISession.FindStringSubmatch(devicePath + "/")
	if m == nil {
		return nexus{}, fmt.Errorf("%w: %s is not an iscsi path", ErrNoNexus, name)
	}
	sid, _ := strconv.Atoi(m[1])
	b, err := os.ReadFile(filepath.Join(sysRoot, "class", "iscsi_session", "session"+m[1], "initiatorname"))
	if err != nil {
		return nexus{}, err
	}
	initiator := strings.TrimSpace(string(b))
	page, err := os.ReadFile(filepath.Join(sysRoot, "block", name, "device", "vpd_pg83"))
	if err != nil {
		return nexus{}, err
	}
	port, ok := relativeTargetPort(page)
	if !ok {
		return nexus{}, fmt.Errorf("%w: %s has no relative target port in its identification page", ErrNoNexus, name)
	}
	return nexus{TransportID: initiator + ",i," + openISCSIISID(sid), RelativeTargetPort: port}, nil
}

// relativeTargetPort returns the relative target port the device
// identification page, vpd 0x83, says: its designator of type 4, associated
// with the target port.
func relativeTargetPort(page []byte) (int, bool) {
	if len(page) < 4 || page[1] != 0x83 {
		return 0, false
	}
	end := 4 + int(binary.BigEndian.Uint16(page[2:4]))
	if end > len(page) {
		end = len(page)
	}
	for i := 4; i+4 <= end; {
		designatorType := page[i+1] & 0x0f
		association := (page[i+1] >> 4) & 0x03
		length := int(page[i+3])
		value := page[i+4 : min(i+4+length, end)]
		if designatorType == 0x4 && association == 0x1 && len(value) == 4 {
			return int(binary.BigEndian.Uint16(value[2:4])), true
		}
		i += 4 + length
	}
	return 0, false
}
