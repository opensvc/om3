package scsi

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/device"
)

type (
	// FullStatusEntry is a registration as a READ FULL STATUS of the
	// persistent reservation says it: its key, the nexus it belongs to,
	// the transport id of the initiator port and the relative target port,
	// and whether it holds the reservation.
	FullStatusEntry struct {
		Key                string
		RelativeTargetPort int
		Holder             bool
		TransportID        string
	}

	// FullStatusReader is a driver reading the full status of the
	// persistent reservation of a device. The command is optional to the
	// targets, and a driver may not support it.
	FullStatusReader interface {
		ReadFullStatus(dev device.T) ([]FullStatusEntry, error)
	}

	// nexus is a current path of the node to a device: the transport id
	// of its initiator port, and its relative target port.
	nexus struct {
		TransportID        string
		RelativeTargetPort int
	}
)

var (
	// ErrNoNexus is the error of the paths of a device the transport of
	// tells no nexus of, as a fibre channel one, whose nexus does not
	// change across a restart.
	ErrNoNexus = errors.New("no nexus known for the paths of the device")

	// readCurrentNexus returns the current paths of the node to a device,
	// set by the tests.
	readCurrentNexus = currentNexus

	regexpFullStatusKey   = regexp.MustCompile(`^\s*Key\s*=\s*(0x[0-9a-fA-F]+)\s*$`)
	regexpFullStatusPort  = regexp.MustCompile(`^\s*Relative port address:\s*0x([0-9a-fA-F]+)`)
	regexpFullStatusISCSI = regexp.MustCompile(`world wide unique port id:\s*(\S+)`)
)

// parseFullStatus reads the registrations of the output of a READ FULL
// STATUS, as sg_persist --in --read-full-status and mpathpersist -i -s write
// it:
//
//	Key=0xdead01
//	  All target ports bit clear
//	  Relative port address: 0xa
//	  << Reservation holder >>
//	  scope: LU_SCOPE,  type: Write Exclusive, registrants only
//	  Transport Id of initiator:
//	    iSCSI world wide unique port id: iqn.2009-11.com.opensvc.srv:n1,i,0x00023d000002
func parseFullStatus(b []byte) []FullStatusEntry {
	l := make([]FullStatusEntry, 0)
	var e *FullStatusEntry
	for _, line := range strings.Split(string(b), "\n") {
		if m := regexpFullStatusKey.FindStringSubmatch(line); m != nil {
			if e != nil {
				l = append(l, *e)
			}
			e = &FullStatusEntry{Key: FormatPRKey(strings.ToLower(m[1]))}
			continue
		}
		if e == nil {
			continue
		}
		switch {
		case strings.Contains(line, "<< Reservation holder >>"):
			e.Holder = true
		case regexpFullStatusPort.MatchString(line):
			m := regexpFullStatusPort.FindStringSubmatch(line)
			if v, err := strconv.ParseInt(m[1], 16, 64); err == nil {
				e.RelativeTargetPort = int(v)
			}
		case regexpFullStatusISCSI.MatchString(line):
			e.TransportID = regexpFullStatusISCSI.FindStringSubmatch(line)[1]
		}
	}
	if e != nil {
		l = append(l, *e)
	}
	return l
}

// sameISCSIPort says two iscsi transport ids name the same initiator port:
// the same initiator name, whatever its case, and the same session id,
// whatever its leading zeroes.
func sameISCSIPort(a, b string) bool {
	nameA, isidA, okA := strings.Cut(a, ",i,")
	nameB, isidB, okB := strings.Cut(b, ",i,")
	if !okA || !okB || !strings.EqualFold(nameA, nameB) {
		return false
	}
	va, errA := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(isidA), "0x"), 16, 64)
	vb, errB := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(isidB), "0x"), 16, 64)
	return errA == nil && errB == nil && va == vb
}

// isCurrent says the registration belongs to a current path of the node.
func (t FullStatusEntry) isCurrent(paths []nexus) bool {
	for _, p := range paths {
		if p.RelativeTargetPort == t.RelativeTargetPort && sameISCSIPort(p.TransportID, t.TransportID) {
			return true
		}
	}
	return false
}

// openISCSIISID returns the session id open-iscsi gives the session number
// sid: 00 02 3d 00, then the session number. It is why the nexus of a path
// changes when the node restarts: the sessions are numbered in login order.
func openISCSIISID(sid int) string {
	return fmt.Sprintf("0x00023d%06x", sid)
}

// checkRegistrationsUnderForeign reports the registrations of our key on a
// device another key holds the reservation of, by the full status of the
// reservation: on a current path of the node, the node has write access to
// the device, as a registration a restart left made live again by a relogin
// given its session id, which is an error; on paths of before only, it is
// harmless until such a relogin, which is a warning. It says nothing when the
// target does not support the full status, or the transport tells no nexus.
func (t *PersistentReservationHandle) checkRegistrationsUnderForeign(dev device.T, reservation string) {
	reader, ok := t.persistentReservationDriver.(FullStatusReader)
	if !ok {
		return
	}
	entries, err := reader.ReadFullStatus(dev)
	if err != nil {
		t.StatusLogger.Info("%s read full status, to tell the paths of the registrations of %s: %s", dev, t.Key, err)
		return
	}
	paths, err := readCurrentNexus(dev)
	if err != nil {
		if !errors.Is(err, ErrNoNexus) {
			t.StatusLogger.Info("%s current paths, to tell the paths of the registrations of %s: %s", dev, t.Key, err)
		}
		return
	}
	var current, stale int
	for _, e := range entries {
		if e.Key != t.Key {
			continue
		}
		if e.isCurrent(paths) {
			current++
		} else {
			stale++
		}
	}
	switch {
	case current > 0:
		t.StatusLogger.Error("%s is reserved by %s and %d current paths of this node are registered with %s: the node has write access to a device another node holds", dev, reservation, current, t.Key)
	case stale > 0:
		t.StatusLogger.Warn("%s is reserved by %s and holds %d registrations of %s of paths of before a restart: a relogin given their session id would have write access to it", dev, reservation, stale, t.Key)
	}
}
