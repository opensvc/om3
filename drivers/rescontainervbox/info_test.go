package rescontainervbox

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVMUUIDFromReader(t *testing.T) {
	out := "name=\"OpenSVC Cluster n2\"\nUUID=\"AEB584E2-B22B-4D7E-B129-AED7A6D446CB\"\nVMState=\"poweroff\"\nmacaddress1=\"08002719DF6C\"\n"
	assert.Equal(t, "aeb584e2-b22b-4d7e-b129-aed7a6d446cb", vmUUIDFromReader(strings.NewReader(out)))
	assert.Empty(t, vmUUIDFromReader(strings.NewReader("name=\"x\"\nVMState=\"running\"\n")))
}
