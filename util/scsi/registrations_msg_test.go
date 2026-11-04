package scsi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The registrations of a device say the local ones against the count
// expected, and the foreign ones, not a count found over a count expected.
func TestRegistrationsMsg(t *testing.T) {
	assert.Equal(t, "2 local registrations (none expected) and 0 foreign", registrationsMsg(2, 0, 0))
	assert.Equal(t, "0 local registrations (none expected) and 2 foreign", registrationsMsg(0, 2, 0))
	assert.Equal(t, "1 local registration (2 expected) and 1 foreign", registrationsMsg(1, 1, 2))
	assert.Equal(t, "2 local registrations and 0 foreign", registrationsMsg(2, 0, -1))
}
