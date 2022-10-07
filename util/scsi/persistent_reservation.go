package scsi

import (
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/xerrors"
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

	PersistentReservationHandle struct {
		Key       string
		Devices   []*device.T
		NoPreempt bool
		Log       *zerolog.Logger
		driver    PersistentReservationDriver
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

func (t *PersistentReservationHandle) StoppedStatus() error {
	if err := t.setup(); err != nil {
		return err
	}
	if len(t.Devices) == 0 {
		return nil
	}
	var errs error
	for _, dev := range t.Devices {
		if reservation, err := t.driver.ReadReservation(*dev); err != nil {
			errs = xerrors.Append(errs, err)
		} else if reservation == t.Key {
			errs = xerrors.Append(errs, errors.Errorf("%s is reserved with local host key %s", dev, reservation, t.Key))
		}
		expectedRegistrationCount := 0
		if registrations, err := t.driver.ReadRegistrations(*dev); err != nil {
			errs = xerrors.Append(errs, err)
		} else if t.countHandledRegistrations(registrations) != expectedRegistrationCount {
			errs = xerrors.Append(errs, errors.Errorf("%d/%d registrations", dev, len(registrations), expectedRegistrationCount))
		}
	}
	return errs
}

func (t *PersistentReservationHandle) StartedStatus() error {
	if err := t.setup(); err != nil {
		return err
	}
	if len(t.Devices) == 0 {
		return nil
	}
	var errs error
	for _, dev := range t.Devices {
		if reservation, err := t.driver.ReadReservation(*dev); err != nil {
			errs = xerrors.Append(errs, err)
		} else if reservation != t.Key {
			errs = xerrors.Append(errs, errors.Errorf("%s is reserved by %s, expected %s", dev, reservation, t.Key))
		}
		expectedRegistrationCount := 2 // TODO
		if registrations, err := t.driver.ReadRegistrations(*dev); err != nil {
			errs = xerrors.Append(errs, err)
		} else if t.countHandledRegistrations(registrations) != expectedRegistrationCount {
			errs = xerrors.Append(errs, errors.Errorf("%d/%d registrations", dev, len(registrations), expectedRegistrationCount))
		}
	}
	return errs
}

func (t *PersistentReservationHandle) setup() error {
	if t.driver != nil {
		return nil
	}
	if capabilities.Has(MpathPersistCapability) {
		t.driver = MpathPersistDriver{
			Log: t.Log,
		}
	} else if capabilities.Has(SGPersistCapability) {
		t.driver = SGPersistDriver{
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
		if err := t.driver.Register(*dev, t.Key); err != nil {
			return err
		}
		if err := t.driver.Reserve(*dev, t.Key); err != nil {
			return err
		}
	}
	return nil
}

func (t *PersistentReservationHandle) Stop() error {
	if err := t.setup(); err != nil {
		return err
	}
	for _, dev := range t.Devices {
		if err := t.driver.Release(*dev, t.Key); err != nil {
			return err
		}
		if err := t.driver.Unregister(*dev, t.Key); err != nil {
			return err
		}
	}
	return nil
}
