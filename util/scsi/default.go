//go:build !linux

package scsi

import "github.com/opensvc/om3/v3/util/capabilities"

func (t *PersistentReservationHandle) setup() error {
	if t.persistentReservationDriver != nil {
		return nil
	}
	if capabilities.Has(SGPersistCapability) {
		t.persistentReservationDriver = SGPersistDriver{
			Log: t.Log,
		}
	} else {
		return ErrNotSupported
	}
	return nil
}
