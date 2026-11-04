package rescontainerocibase

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A process in the network namespace of the node, as a container run with
// --net host, has no namespace of its own to plug an address in.
func TestTheNetNSOfAProcessInTheNodeNetNSIsNotPrivate(t *testing.T) {
	assert.Equal(t, "", privateNetNSPathOf(os.Getpid()))
}

// A process gone has no namespace.
func TestTheNetNSOfAProcessGoneIsNone(t *testing.T) {
	assert.Equal(t, "", privateNetNSPathOf(1<<30))
}
