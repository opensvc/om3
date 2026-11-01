package rescontainerkvm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/claim"
)

type fakeCapper struct {
	limit int64
	err   error
}

func (t fakeCapper) ResourceCap(string, string) (int64, error) { return t.limit, t.err }

// A node libvirt places the guests of out of the process groups is refused a
// guest the object caps, not only one the namespace claims for.
func TestCappedError(t *testing.T) {
	assert.NoError(t, cappedError(nil, "container#1", claim.TypeMemory), "no object to ask")
	assert.NoError(t, cappedError(fakeCapper{limit: claim.Unbounded}, "container#1", claim.TypeMemory))

	err := cappedError(fakeCapper{limit: 1024}, "container#1", claim.TypeCPU)
	assert.ErrorContains(t, err, "container#1 is capped on the "+claim.TypeCPU)

	failed := errors.New("failed")
	assert.ErrorIs(t, cappedError(fakeCapper{err: failed}, "container#1", claim.TypeMemory), failed)
}
