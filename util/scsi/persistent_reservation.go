package scsi

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/plog"
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
		CurrentStatus		    PersistentReservationStatus
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

// formatKey formats a string key by stripping a leading "0x", truncating it to 16 characters if necessary,
// and padding it with leading zeroes. The formatted key is returned with a "0x" prefix.
// It is used to format the reservation or registration key read from device and the
// configured node scsi key.
//
//	"0xa"                   -> "0x000000000000000a"
//	"0xab"                  -> "0x00000000000000ab"
//	"0x1234567890123456789" -> "0x1234567890123456"
//	"0x1234567890123456"    -> "0x1234567890123456"
func formatKey(s string) string {
	// Strip leading 0x
	s = s[2:]
	if len(s) > 16 {
		// Truncate to 16 bytes
		s = s[:16]
	}
	// add leading 0x and Pad with 0
	return fmt.Sprintf("0x%016s", s)
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
		} else {
			reservationMsg = fmt.Sprintf("%s is reserved", dev)
			s = status.Up
		}
	}

	var expectedRegistrationCount int
	if s == status.Up {
		expectedRegistrationCount, err = t.DeviceExpectedRegistrationCount(dev)
		if err != nil {
			t.StatusLogger.Error("%s expected registration count: %s", dev, err)
			return status.NotApplicable
		}
	}

	if registrations, err := t.persistentReservationDriver.ReadRegistrations(dev); err != nil {
		t.StatusLogger.Error("%s read registrations: %s", dev, err)
		s = status.Undef
	} else if handledRegistrationCount := t.countHandledRegistrations(registrations); handledRegistrationCount == expectedRegistrationCount {
		if expectedRegistrationCount == 0 {
			if handledRegistrationCount == 0 {
				t.StatusLogger.Info("%s, no registrations", reservationMsg)
			} else {
				t.StatusLogger.Warn("%s, %d/%d registrations", reservationMsg, handledRegistrationCount, expectedRegistrationCount)
				s.Add(status.Warn)
			}
		} else {
			t.StatusLogger.Info("%s, %d/%d registrations", reservationMsg, handledRegistrationCount, expectedRegistrationCount)
			s.Add(status.Up)
		}
	} else {
		t.StatusLogger.Warn("%s, %d/%d registrations", reservationMsg, handledRegistrationCount, expectedRegistrationCount)
		s.Add(status.Warn)
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
		deviceStatus := t.DeviceStatus(dev)
		if t.CurrentStatus.ReservedBy != nil && *t.CurrentStatus.ReservedBy != "" && *t.CurrentStatus.ReservedBy != t.Key && !t.Force {
			return fmt.Errorf("%s is already reserved by the foreign key %s: use --force to preempt if you are sure the key owner can be fenced", dev, *t.CurrentStatus.ReservedBy)
		}
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
			// already reserved
		} else if reservation == "" {
			// not reserved or preempt not allowed => Reserve action
			if err := t.persistentReservationDriver.Reserve(dev, t.Key); err != nil {
				return fmt.Errorf("%s spr reserve: %w", dev.Path(), err)
			}
		} else if t.NoPreemptAbort {
			if err := t.persistentReservationDriver.Preempt(dev, reservation, t.Key); err != nil {
				return fmt.Errorf("%s spr preempt (no_preempt_abort kw): %w", dev.Path(), err)
			}
		} else if vendor, _ := dev.Vendor(); vendor == "VMware" {
			if err := t.persistentReservationDriver.Preempt(dev, reservation, t.Key); err != nil {
				return fmt.Errorf("%s spr preempt (VMware vdisk quirk): %w", dev.Path(), err)
			}
		} else {
			if err := t.persistentReservationDriver.PreemptAbort(dev, reservation, t.Key); err != nil {
				return fmt.Errorf("%s spr preempt-abort: %w", dev.Path(), err)
			}
		}
	}
	return nil
}

func (t *PersistentReservationHandle) Stop() error {
	if err := t.setup(); err != nil {
		return err
	}
	for _, dev := range t.Devices {
		if s := t.DeviceStatus(dev); s == status.Down {
			t.Log.Infof("%s is already unregistered and unreserved", dev)
			continue
		}
		if err := t.persistentReservationDriver.Release(dev, t.Key); err != nil {
			return fmt.Errorf("%s spr release: %w", dev.Path(), err)
		}
		if err := t.persistentReservationDriver.Unregister(dev, t.Key); err != nil {
			return fmt.Errorf("%s spr unregister: %w", dev.Path(), err)
		}
	}
	return nil
}
