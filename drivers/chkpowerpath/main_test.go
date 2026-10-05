package chkpowerpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const display = `Pseudo name=emcpowerh
Symmetrix ID=000290101523
Logical device ID=17C6
state=alive; policy=SymmOpt; priority=0; queued-IOs=0
==============================================================================
---------------- Host ---------------   - Stor -   -- I/O Path -  -- Stats ---
###  HW Path                I/O Paths    Interf.   Mode    State  Q-IOs Errors
==============================================================================
   0 qla2xxx                   sdi       FA  9dB   active  alive      0      0
   1 qla2xxx                   sds       FA  8dB   active  dead       0      1

Pseudo name=emcpoweri
   0 qla2xxx                   sdj       FA  9dB   active  alive      0      0
`

func TestParse(t *testing.T) {
	assert.Equal(t, []pseudo{
		{name: "emcpowerh", paths: []string{"/dev/sdi", "/dev/sds"}, activePaths: 1},
		{name: "emcpoweri", paths: []string{"/dev/sdj"}, activePaths: 1},
	}, parse([]byte(display)))
}
