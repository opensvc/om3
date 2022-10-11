package scsi

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/device"
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
		Devices                     []*device.T
		NoPreempt                   bool
		Log                         *zerolog.Logger
		StatusLogger                statusLogger
		persistentReservationDriver PersistentReservationDriver
	}
)

var (
	DefaultPersistentReservationType   = "5" // Write-Exclusive Registrants-Only
	DefaultPersistentReservationDriver = MpathPersistDriver{}
	ErrNotSupported                    = errors.New("SCSI PR is not supported on this node: no usable mpathpersist or sg_persist")
)

func MakePRKey() []byte {
	return uuid.NodeID()
}

func (t PersistentReservationHandle) countHandledRegistrations(registrations []string) int {
	n := 0
	for _, r := range registrations {
		if r == t.Key {
			n += 1
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
		if dev == nil {
			continue
		}
		s := t.DeviceStatus(*dev)
		agg.Add(s)
	}
	return agg
}

func (t *PersistentReservationHandle) DeviceStatus(dev device.T) status.T {
	var reservationMsg string
	s := status.Down
	_, err := os.Stat(dev.Path())
	switch {
	case os.IsNotExist(err):
		t.StatusLogger.Info("%s is not reservable: does not exist", dev)
		return status.NotApplicable
	case err != nil:
		t.StatusLogger.Error("%s exist: %s", dev, err)
	}
	if v, err := dev.IsSCSI(); err != nil {
		t.StatusLogger.Error("%s is scsi: %s", dev, err)
	} else if !v {
		t.StatusLogger.Info("%s is not reservable: not a scsi device", dev)
		return status.NotApplicable
	}
	if reservation, err := t.persistentReservationDriver.ReadReservation(dev); err != nil {
		t.StatusLogger.Error("%s read reservation: %s", dev, err)
	} else if reservation == "" {
		reservationMsg = fmt.Sprintf("%s is not reserved", dev)
	} else if reservation != t.Key {
		reservationMsg = fmt.Sprintf("%s is reserved by %s", dev, reservation)
	} else {
		reservationMsg = fmt.Sprintf("%s is reserved", dev)
		s = status.Up
	}

	var expectedRegistrationCount int
	if s == status.Up {
		expectedRegistrationCount = 2 // TODO: real count
	}

	if registrations, err := t.persistentReservationDriver.ReadRegistrations(dev); err != nil {
		t.StatusLogger.Error("%s read registrations: %s", dev, err)
		s = status.Undef
	} else if t.countHandledRegistrations(registrations) != expectedRegistrationCount {
		t.StatusLogger.Warn("%s, %d/%d registrations", reservationMsg, len(registrations), expectedRegistrationCount)
		s.Add(status.Warn)
	} else {
		t.StatusLogger.Info("%s, %d/%d registrations", reservationMsg, len(registrations), expectedRegistrationCount)
		if expectedRegistrationCount > 0 {
			s.Add(status.Up)
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

func (t *PersistentReservationHandle) setup() error {
	if t.persistentReservationDriver != nil {
		return nil
	}
	if capabilities.Has(MpathPersistCapability) {
		t.persistentReservationDriver = MpathPersistDriver{
			Log: t.Log,
		}
	} else if capabilities.Has(SGPersistCapability) {
		t.persistentReservationDriver = SGPersistDriver{
			Log: t.Log,
		}
	} else {
		return ErrNotSupported
	}
	return nil
}

func (t *PersistentReservationHandle) Start() error {
	if err := t.setup(); err != nil {
		return err
	}
	for _, dev := range t.Devices {
		if s := t.DeviceStatus(*dev); s == status.Up {
			t.Log.Info().Msgf("%s is already registered and reserved", dev)
			continue
		}
		if err := t.persistentReservationDriver.Register(*dev, t.Key); err != nil {
			return errors.Wrapf(err, "%s spr register", dev.Path())
		}
		if err := t.persistentReservationDriver.Reserve(*dev, t.Key); err != nil {
			return errors.Wrapf(err, "%s spr reserve", dev.Path())
		}
	}
	return nil
}

func (t *PersistentReservationHandle) Stop() error {
	if err := t.setup(); err != nil {
		return err
	}
	for _, dev := range t.Devices {
		if s := t.DeviceStatus(*dev); s == status.Down {
			t.Log.Info().Msgf("%s is already unregistered and unreserved", dev)
			continue
		}
		if err := t.persistentReservationDriver.Release(*dev, t.Key); err != nil {
			return errors.Wrapf(err, "%s spr release", dev.Path())
		}
		if err := t.persistentReservationDriver.Unregister(*dev, t.Key); err != nil {
			return errors.Wrapf(err, "%s spr unregister", dev.Path())
		}
	}
	return nil
}
