package resfshost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The name wipefs gives a backup says the offset of the signature, which the
// dd restoring it seeks to.
func TestWipeBackupNameSaysTheOffset(t *testing.T) {
	m := wipeBackupName.FindStringSubmatch("wipefs-loop1-0x00000438.bak")
	require.NotNil(t, m)
	assert.Equal(t, "00000438", m[1])
	m = wipeBackupName.FindStringSubmatch("wipefs-mapper-data-v-0x00010040.bak")
	require.NotNil(t, m)
	assert.Equal(t, "00010040", m[1])
	assert.Nil(t, wipeBackupName.FindStringSubmatch("notes.txt"))
}
