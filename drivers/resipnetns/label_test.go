package resipnetns

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The node not running the instance holds no address drawn from a network:
// the label names the network, with no <nil> address nor mask.
func TestTheLabelOfAnAddressNotTakenNamesItsNetwork(t *testing.T) {
	assert.Equal(t, "backend in container#0", label("backend", "", nil, 0, "", "container#0"))
	assert.Equal(t, "backend@eth1 in container#0", label("backend", "", nil, 0, "eth1", "container#0"))
	assert.Equal(t, "web.example.com in container#0", label("", "web.example.com", nil, 0, "", "container#0"))
}

func TestTheLabelOfAnAddressTakenShowsIt(t *testing.T) {
	ip := net.ParseIP("10.139.0.59")
	assert.Equal(t, "backend 10.139.0.59/24 in container#0", label("backend", "", ip, 24, "", "container#0"))
	assert.Equal(t, "10.139.0.59/24@eth1 in container#0", label("", "", ip, 24, "eth1", "container#0"))
}
