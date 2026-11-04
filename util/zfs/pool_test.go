package zfs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A name no pool can have, as one zfs reserves, is a pool that does not
// exist: the resource reads down, and its purge needs no force. Another
// failure says nothing of the pool, and tells why.
func TestPoolExists(t *testing.T) {
	exitErr := errors.New("zpool exit code 1 not in success codes: [0]")
	for stderr, absent := range map[string]bool{
		"cannot open 'p1': no such pool\n":                        true,
		"cannot open 'c13pool': name is reserved\n":               true,
		"cannot open 'a/b': invalid character '/' in pool name\n": true,
		"cannot open '1p': name must begin with a letter\n":       true,
		"cannot open 'pppp': name is too long\n":                  true,
		"The ZFS modules are not loaded.\n":                       false,
	} {
		exists, err := poolExists(exitErr, []byte(stderr))
		assert.False(t, exists, stderr)
		if absent {
			assert.NoError(t, err, stderr)
		} else {
			assert.ErrorIs(t, err, exitErr, stderr)
			assert.ErrorContains(t, err, "The ZFS modules are not loaded.", "the reason is told")
		}
	}
	exists, err := poolExists(nil, nil)
	assert.True(t, exists)
	assert.NoError(t, err)
}
