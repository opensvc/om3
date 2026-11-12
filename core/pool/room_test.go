package pool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A pool whose status failed on a node, as a share not mounted there, has no
// room: its usage of zero read as the one of a pool with no size of its own,
// whose volumes take of other pools, and the pool served volumes the other
// nodes do not see.
func TestAPoolWhoseStatusFailedHasNoRoom(t *testing.T) {
	byNode := map[string]map[string]Usage{
		"n1": {"nfs": {Errors: []string{"the share /srv/share is not mounted: it is on a local filesystem (statfs type 0xef53)"}}},
		"n2": {"nfs": {Size: 100, Free: 100}},
	}
	assert.Equal(t, "[nfs] node n1: the share /srv/share is not mounted: it is on a local filesystem (statfs type 0xef53)", roomCauseOf("nfs", []string{"n1", "n2"}, byNode, 10))
	assert.Empty(t, roomCauseOf("nfs", []string{"n2"}, byNode, 10))

	byNode["n1"]["nfs"] = Usage{}
	assert.Empty(t, roomCauseOf("nfs", []string{"n1"}, byNode, 10), "a pool with no size of its own, a virtual one, says nothing of room")
}

// A pool shared by the nodes has its room checked once, but its status on
// every node: a node whose share is not mounted was not looked at once the
// first node said the shared pool had room.
func TestASharedPoolIsCheckedOnEveryNode(t *testing.T) {
	byNode := map[string]map[string]Usage{
		"n1": {"nfs": {Shared: true, Size: 100, Free: 100}},
		"n2": {"nfs": {Shared: true, Errors: []string{"the share /srv/share is not mounted"}}},
	}
	assert.Equal(t, "[nfs] node n2: the share /srv/share is not mounted", roomCauseOf("nfs", []string{"n1", "n2"}, byNode, 10))

	byNode["n2"]["nfs"] = Usage{Shared: true, Size: 100, Free: 5}
	assert.Empty(t, roomCauseOf("nfs", []string{"n1", "n2"}, byNode, 10), "the room of one storage is the one the first node saw")

	delete(byNode["n2"], "nfs")
	byNode["n2"]["other"] = Usage{Size: 1}
	assert.Equal(t, "[nfs] node n2 has no such pool", roomCauseOf("nfs", []string{"n1", "n2"}, byNode, 10))
}
