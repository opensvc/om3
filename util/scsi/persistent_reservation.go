package scsi

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	PersistentReservationDriver interface {
		ReadRegistrations(dev device.T) ([]string, error)
		Register(dev device.T, key string) error
		Unregister(dev device.T, key string) error
		ReadReservation(dev device.T) (string, error)
		Reserve(dev device.T, key string) error
		Release(dev device.T, key string) error
		Clear(dev device.T, key string) error
		Preempt(dev device.T, oldKey, newKey string) error
		PreemptAbort(dev device.T, oldKey, newKey string) error
	}

	statusLogger interface {
		Info(s string, args ...any)
		Warn(s string, args ...any)
		Error(s string, args ...any)
	}

	PersistentReservationHandle struct {
		Key                         string
		Devices                     device.L
		NoPreemptAbort              bool
		Force                       bool
		Log                         *plog.Logger
		StatusLogger                statusLogger
		persistentReservationDriver PersistentReservationDriver
		CurrentStatus               PersistentReservationStatus
		CoresourceDisabled          bool
		CoresourceStatus            status.T

		// ForeignReserved are the devices another key holds the
		// reservation of, as the last Status found them.
		ForeignReserved []string

		// expectedRegistrationCount, set by a test, replaces
		// DeviceExpectedRegistrationCount, which reads the sysfs of the
		// device.
		expectedRegistrationCount func(device.T) (int, error)
	}

	PersistentReservationStatus struct {
		ReservedBy *string
	}
)

var (
	DefaultPersistentReservationType = "5" // Write-Exclusive Registrants-Only
	ErrNotSupported                  = errors.New("the SCSI PR is not supported on this node: no usable mpathpersist or sg_persist")
)

func StripPRKey(s string) string {
	return "0x" + strings.ToLower(strings.TrimLeft(strings.TrimPrefix(s, "0x"), "0"))
}

// FormatPRKey formats a string key by stripping a leading "0x", truncating it to 16 characters if necessary,
// and padding it with leading zeroes. The formatted key is returned with a "0x" prefix.
// It is used to format the reservation or registration key read from device and the
// configured node scsi key.
//
//	"0xa"                   -> "0x000000000000000a"
//	"0xab"                  -> "0x00000000000000ab"
//	"0x1234567890123456789" -> "0x1234567890123456"
//	"0x1234567890123456"    -> "0x1234567890123456"
func FormatPRKey(s string) string {
	// Strip leading 0x
	s = s[2:]
	if len(s) > 16 {
		// Truncate to 16 bytes
		s = s[:16]
	}
	// add leading 0x and Pad with 0
	return fmt.Sprintf("0x%016s", s)
}

// registrationsMsg says the registrations of a device: those of the local
// key against the count expected, -1 when nothing tells what to expect, and
// those of other keys. A count found against a count expected, as "2/0",
// read as a count gone wrong.
func registrationsMsg(local, foreign, expected int) string {
	s := fmt.Sprintf("%d local %s", local, plural(local, "registration", "registrations"))
	switch {
	case expected == 0:
		s += " (none expected)"
	case expected > 0:
		s += fmt.Sprintf(" (%d expected)", expected)
	}
	return s + fmt.Sprintf(" and %d foreign", foreign)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (t PersistentReservationHandle) countHandledRegistrations(registrations []string) int {
	n := 0
	for _, r := range registrations {
		if r == t.Key {
			n++
		}
	}
	return n
}

func (t *PersistentReservationHandle) Status() status.T {
	if err := t.setup(); err != nil {
		t.StatusLogger.Error("%s", err)
		return status.Undef
	}
	t.ForeignReserved = nil
	if len(t.Devices) == 0 {
		return status.NotApplicable
	}
	agg := status.Undef
	for _, dev := range t.Devices {
		s := t.DeviceStatus(dev)
		agg.Add(s)
	}
	return agg
}

func (t *PersistentReservationHandle) DeviceExpectedRegistrationCount(dev device.T) (int, error) {
	pathCount := func(dev device.T) (int, error) {
		if slaves, err := dev.Slaves(); err != nil {
			return 0, err
		} else {
			return len(slaves), nil
		}
	}
	hostCount := func(dev device.T) (int, error) {
		if hosts, err := dev.SlaveHosts(); err != nil {
			return 0, err
		} else {
			return len(hosts), nil
		}
	}
	if v, err := dev.IsMultipath(); err != nil {
		return 0, err
	} else if v {
		vendor, err := dev.Vendor()
		if err != nil {
			return 0, err
		}
		model, err := dev.Model()
		if err != nil {
			return 0, err
		}
		switch {
		case (vendor == "3PARdata") && (model == "VV"):
			// 3PARdata arrays transparent controller failover has S3GPR quirks.
			// All registration via the same I are consider the same I_T, so
			// the expected registration count is the number of I instead of the
			// number of I_T.
			return hostCount(dev)
		default:
			return pathCount(dev)
		}
	}
	if v, err := dev.IsSCSI(); err != nil {
		return 0, err
	} else if v {
		return 1, nil
	}
	return 0, nil
}

func (t *PersistentReservationHandle) DeviceStatus(dev device.T) status.T {
	var reservationMsg string
	t.CurrentStatus = PersistentReservationStatus{}
	s := status.Down
	_, err := os.Stat(dev.Path())
	switch {
	case os.IsNotExist(err):
		t.StatusLogger.Info("%s is not reservable: does not exist", dev)
		return status.NotApplicable
	case err != nil:
		t.StatusLogger.Error("%s exist: %s", dev, err)
	}
	if v, err := dev.IsReservable(); err != nil {
		t.StatusLogger.Error("%s is reservable? %s", dev, err)
	} else if !v {
		t.StatusLogger.Info("%s is not reservable: not a scsi or mpath device", dev)
		return status.NotApplicable
	}
	if reservation, err := t.persistentReservationDriver.ReadReservation(dev); err != nil {
		t.StatusLogger.Error("%s read reservation: %s", dev, err)
	} else {
		t.CurrentStatus.ReservedBy = &reservation
		if reservation == "" {
			reservationMsg = fmt.Sprintf("%s is not reserved", dev)
		} else if reservation != t.Key {
			reservationMsg = fmt.Sprintf("%s is reserved by %s", dev, reservation)
			t.ForeignReserved = append(t.ForeignReserved, dev.String())
		} else {
			reservationMsg = fmt.Sprintf("%s is reserved", dev)
			s = status.Up
		}
	}

	// The co-resource witnesses whether the instance holds the device: up,
	// it does, and every path must be registered, down, it does not, and
	// none may be. A co-resource n/a, like a disk that is there whatever the
	// state of the instance, witnesses nothing, and the reservation does
	// instead: held with our key, the instance holds the device. Not held,
	// nothing tells whether the instance is started, so registrations
	// missing, as on a node standing by, or left over, are not an issue to
	// report.
	holdsDevice := t.CoresourceStatus == status.Up
	witnessed := true
	if t.CoresourceStatus == status.NotApplicable && !t.CoresourceDisabled {
		holdsDevice = s == status.Up
		witnessed = holdsDevice
	}

	var expectedRegistrationCount int
	if holdsDevice {
		expectedRegistrationCount, err = t.DeviceExpectedRegistrationCount(dev)
		if err != nil {
			t.StatusLogger.Error("%s expected registration count: %s", dev, err)
			return status.NotApplicable
		}
	}

	if registrations, err := t.persistentReservationDriver.ReadRegistrations(dev); err != nil {
		t.StatusLogger.Error("%s read registrations: %s", dev, err)
		s = status.Undef
	} else {
		local := t.countHandledRegistrations(registrations)
		foreign := len(registrations) - local
		switch {
		case !witnessed:
			t.StatusLogger.Info("%s, %s", reservationMsg, registrationsMsg(local, foreign, -1))
		case local == expectedRegistrationCount:
			t.StatusLogger.Info("%s, %s", reservationMsg, registrationsMsg(local, foreign, expectedRegistrationCount))
			if local > 0 {
				s.Add(status.Up)
			}
		default:
			t.StatusLogger.Warn("%s, %s", reservationMsg, registrationsMsg(local, foreign, expectedRegistrationCount))
			s.Add(status.Warn)
		}
	}

	// Report n/a instead of up for scsireserv status if a dev is ro
	//
	// Because in this case, we can't clear the reservation until the dev is
	// promoted rw.
	//
	// This happens on srdf devices that became r2 after a failover due to a crash.
	// When the crashed node comes up again, the reservation are still held on the
	// r2 devices, and they can't be dropped.
	//
	// Still report the situation as a resource log "info" message.
	if s == status.Up {
		if v, err := dev.IsReadOnly(); err != nil {
			t.StatusLogger.Error("%s %s", dev, err)
		} else if v {
			t.StatusLogger.Info("%s is read-only")
			s = status.NotApplicable
		}
	}
	return s
}

func (t *PersistentReservationHandle) Start() error {
	if err := t.setup(); err != nil {
		return err
	}
	for _, dev := range t.Devices {
		// Read here, and not taken from the status: a read that failed
		// leaves the status saying nothing of the reservation, and a
		// reservation another key holds is not to be found out by taking it.
		if reservation, err := t.persistentReservationDriver.ReadReservation(dev); err != nil {
			return fmt.Errorf("%s read reservation, to check no foreign key holds it: %w", dev, err)
		} else if err := t.checkForeign(dev, reservation); err != nil {
			return err
		}
		deviceStatus := t.DeviceStatus(dev)
		if deviceStatus == status.Up {
			t.Log.Infof("%s is already registered and reserved", dev)
			continue
		}
		if err := t.persistentReservationDriver.Register(dev, t.Key); err != nil {
			return fmt.Errorf("%s spr register: %w", dev.Path(), err)
		}

		if reservation, err := t.persistentReservationDriver.ReadReservation(dev); err != nil {
			return err
		} else if reservation == t.Key {
			// Already reserved, maybe by a path of the node registered
			// before it restarted: see dropStaleRegistrations.
			if err := t.dropStaleRegistrations(dev); err != nil {
				return err
			}
		} else if reservation == "" {
			// not reserved or preempt not allowed => Reserve action
			if err := t.persistentReservationDriver.Reserve(dev, t.Key); err != nil {
				return fmt.Errorf("%s spr reserve: %w", dev.Path(), err)
			}
		} else if err := t.checkForeign(dev, reservation); err != nil {
			// Reserved by another key since the check, as by a node
			// starting at the same time: not taking it, and leaving the
			// device as it was found.
			if unregErr := t.persistentReservationDriver.Unregister(dev, t.Key); unregErr != nil {
				return errors.Join(err, fmt.Errorf("%s spr unregister: %w", dev.Path(), unregErr))
			}
			return err
		} else if err := t.preempt(dev, reservation); err != nil {
			return err
		}
	}
	return nil
}

// preempt takes the reservation of the device from the key holding it, and
// drops the registrations of that key but the issuing path's: a preempt and
// abort, which also aborts the commands the preempted paths still have
// pending on the device, so a node taken over can not write after the
// takeover. A plain preempt where no_preempt_abort says so, and on the
// VMware vdisks, which refuse the abort.
//
// The own key is preempted the same way, to take the reservation and drop
// the registrations of the paths of before a node restart: see Stop and
// dropStaleRegistrations.
func (t *PersistentReservationHandle) preempt(dev device.T, holder string) error {
	if t.NoPreemptAbort {
		if err := t.persistentReservationDriver.Preempt(dev, holder, t.Key); err != nil {
			return fmt.Errorf("%s spr preempt of %s (no_preempt_abort kw): %w", dev.Path(), holder, err)
		}
		return nil
	}
	if vendor, _ := dev.Vendor(); vendor == "VMware" {
		if err := t.persistentReservationDriver.Preempt(dev, holder, t.Key); err != nil {
			return fmt.Errorf("%s spr preempt of %s (VMware vdisk quirk): %w", dev.Path(), holder, err)
		}
		return nil
	}
	if err := t.persistentReservationDriver.PreemptAbort(dev, holder, t.Key); err != nil {
		return fmt.Errorf("%s spr preempt-abort of %s: %w", dev.Path(), holder, err)
	}
	return nil
}

// checkForeign refuses a reservation held by another key, unless forced.
func (t *PersistentReservationHandle) checkForeign(dev device.T, reservation string) error {
	if reservation == "" || reservation == t.Key || t.Force {
		return nil
	}
	return fmt.Errorf("%s is already reserved by the foreign key %s: use --force to preempt if you are sure the key owner can be fenced", dev, reservation)
}

// Stop releases the reservation the node holds on the devices, and
// unregisters the paths of the node.
//
// A registration, and a reservation, belong to a path, an I_T nexus, and the
// nexus of an iscsi path changes when the node restarts, its initiator
// session ids given in login order: after a restart, the paths of the node
// are not the ones registered, and a release from them was refused, failing
// the stop and the unprovision of every instance not started since. The paths
// are registered before the release, and a reservation of our key still held
// after it, by a path of before, is preempted from a current path, which
// takes it and drops the registrations of our key the other paths hold, so
// the release works.
//
// A reservation another key holds is left alone, and no path is registered
// then: a registration would give the node the write access to a device
// another node holds. The registrations of our key left there are the ones
// of paths of before, or the holder took them away.
func (t *PersistentReservationHandle) Stop() error {
	if err := t.setup(); err != nil {
		return err
	}
	for _, dev := range t.Devices {
		if err := t.stopDevice(dev); err != nil {
			return err
		}
	}
	return nil
}

func (t *PersistentReservationHandle) stopDevice(dev device.T) error {
	reservation, err := t.persistentReservationDriver.ReadReservation(dev)
	if err != nil {
		return fmt.Errorf("%s read reservation: %w", dev, err)
	}
	if reservation != "" && reservation != t.Key {
		t.Log.Infof("%s is reserved by the foreign key %s: the reservation is left, the paths unregistered", dev, reservation)
		if err := t.persistentReservationDriver.Unregister(dev, t.Key); err != nil {
			t.Log.Warnf("%s spr unregister, under the reservation of another key: %s", dev.Path(), err)
		}
		return nil
	}
	local, err := t.localRegistrations(dev)
	if err != nil {
		return err
	}
	if reservation == "" && local == 0 {
		t.Log.Infof("%s is already unregistered and unreserved", dev)
		return nil
	}
	if reservation == t.Key {
		// The paths of the node may not be the ones registered, after
		// it restarted: a release from a path not registered is refused.
		if err := t.persistentReservationDriver.Register(dev, t.Key); err != nil {
			return fmt.Errorf("%s spr register, to release: %w", dev.Path(), err)
		}
		if err := t.persistentReservationDriver.Release(dev, t.Key); err != nil {
			t.Log.Infof("%s spr release: %s", dev.Path(), err)
		}
		if reservation, err = t.persistentReservationDriver.ReadReservation(dev); err != nil {
			return fmt.Errorf("%s read reservation, after the release: %w", dev, err)
		}
		if reservation == t.Key {
			// Held by a path registered before the node restarted,
			// which a release from a current path leaves.
			t.Log.Infof("%s is still reserved by %s, held by a path of before: preempt the own key to release it", dev, t.Key)
			if err := t.preempt(dev, t.Key); err != nil {
				return fmt.Errorf("preempt the own key, to release: %w", err)
			}
			if err := t.persistentReservationDriver.Release(dev, t.Key); err != nil {
				return fmt.Errorf("%s spr release, after the preempt of the own key: %w", dev.Path(), err)
			}
			if reservation, err = t.persistentReservationDriver.ReadReservation(dev); err != nil {
				return fmt.Errorf("%s read reservation, after the release: %w", dev, err)
			} else if reservation == t.Key {
				return fmt.Errorf("%s is still reserved by %s after a preempt and a release", dev, t.Key)
			}
		}
	}
	if err := t.persistentReservationDriver.Unregister(dev, t.Key); err != nil {
		// Another node took the device meanwhile, as one starting,
		// whose preempt removed the registrations of our key: there is
		// nothing left to unregister.
		if t.releasedByAnother(dev) {
			t.Log.Infof("%s spr unregister: %s: the device is reserved by another key and holds no registration of ours", dev.Path(), err)
			return nil
		}
		return fmt.Errorf("%s spr unregister: %w", dev.Path(), err)
	}
	if reservation == "" {
		return t.dropRegistrationsOfBefore(dev)
	}
	return nil
}

// dropRegistrationsOfBefore drops the registrations of our key a device not
// reserved holds after the unregistration of the current paths, the ones of
// paths of before a restart: a preempt of the own key from a current path
// drops them, and the current path is unregistered after it.
//
// The reservation is read again first: a peer may have reserved the device
// since, and a path registered under its reservation would have the write
// access to it. The registrations of paths of before are left then, harmless,
// and the holder drops them when it preempts our key. A failure leaving a
// current path registered fails the stop.
func (t *PersistentReservationHandle) dropRegistrationsOfBefore(dev device.T) error {
	n, err := t.localRegistrations(dev)
	if err != nil || n == 0 {
		return err
	}
	reservation, err := t.persistentReservationDriver.ReadReservation(dev)
	if err != nil {
		return fmt.Errorf("%s read reservation, to drop the registrations of paths of before: %w", dev, err)
	}
	if reservation != "" {
		t.Log.Infof("%s holds %d registrations of %s of paths of before, and is reserved by %s since: left", dev, n, t.Key, reservation)
		return nil
	}
	t.Log.Infof("%s holds %d registrations of %s after the unregistration, of paths of before: preempt them", dev, n, t.Key)
	var errs error
	if err := t.persistentReservationDriver.Register(dev, t.Key); err != nil {
		errs = fmt.Errorf("%s spr register, to drop the registrations of paths of before: %w", dev.Path(), err)
	} else if err := t.preempt(dev, t.Key); err != nil {
		errs = fmt.Errorf("preempt the own key, to drop the registrations of paths of before: %w", err)
	}
	// The current paths are unregistered whatever failed before: none is
	// to stay registered.
	if err := t.persistentReservationDriver.Unregister(dev, t.Key); err != nil {
		return errors.Join(errs, fmt.Errorf("%s spr unregister: %w", dev.Path(), err))
	}
	if errs != nil {
		t.Log.Warnf("%s: the registrations of paths of before are left: %s", dev, errs)
	}
	return nil
}

// releasedByAnother says the device is reserved by another key and holds no
// registration of ours: what the stop was to reach, another node having
// taken the device.
func (t *PersistentReservationHandle) releasedByAnother(dev device.T) bool {
	reservation, err := t.persistentReservationDriver.ReadReservation(dev)
	if err != nil || reservation == "" || reservation == t.Key {
		return false
	}
	n, err := t.localRegistrations(dev)
	return err == nil && n == 0
}

// localRegistrations returns the count of the registrations of our key.
func (t *PersistentReservationHandle) localRegistrations(dev device.T) (int, error) {
	registrations, err := t.persistentReservationDriver.ReadRegistrations(dev)
	if err != nil {
		return 0, fmt.Errorf("%s read registrations: %w", dev, err)
	}
	return t.countHandledRegistrations(registrations), nil
}

// dropStaleRegistrations moves the reservation of our key to a current path,
// and drops the registrations of our key the paths of the node no longer
// use, when there are more of them than the node has paths, as after a
// restart that changed the nexus of its iscsi paths: the reservation stayed
// held by a path of before, the device warned of the registrations in excess
// for good, and a stop then left it reserved.
//
// A preempt of our own key, from a current path, removes every registration
// of our key but the one of that path, and moves our reservation to it. The
// other current paths are registered again after it. A reservation of
// another key is never touched: the preempt names our key.
func (t *PersistentReservationHandle) dropStaleRegistrations(dev device.T) error {
	local, err := t.localRegistrations(dev)
	if err != nil {
		return err
	}
	countExpected := t.DeviceExpectedRegistrationCount
	if t.expectedRegistrationCount != nil {
		countExpected = t.expectedRegistrationCount
	}
	expected, err := countExpected(dev)
	if err != nil || expected <= 0 || local <= expected {
		return nil
	}
	t.Log.Infof("%s holds %d registrations of %s for %d paths: preempt the own key to drop the ones of paths of before", dev, local, t.Key, expected)
	if err := t.preempt(dev, t.Key); err != nil {
		return fmt.Errorf("preempt the own key, to drop the registrations of paths of before: %w", err)
	}
	if err := t.persistentReservationDriver.Register(dev, t.Key); err != nil {
		return fmt.Errorf("%s spr register, after the preempt of the own key: %w", dev.Path(), err)
	}
	return nil
}
