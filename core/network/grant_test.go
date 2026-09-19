package network

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Two resources reserving at once are each answered as though the other had
// not, where the answers come from the addresses the cluster reports alone.
func TestAGrantIsWeighedAgainstTheNextClaim(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	seen := map[string]bool{
		"ns/svc/s1!ip#1": true,
		"ns/svc/s2!ip#1": true,
	}
	ok, _ := grants.Fits("ns", "n", "ns/svc/s3!ip#1", 3, 2, seen, now)
	assert.True(t, ok, "the third address fits the claim")

	ok, why := grants.Fits("ns", "n", "ns/svc/s4!ip#1", 3, 2, seen, now)
	assert.False(t, ok, "the fourth does not, and the third is why")
	assert.Contains(t, why, "already holds 3")
}

// A claim asked twice for the same resource is the same address twice.
func TestAskingTwiceForTheSameReservationIsAnsweredTheSame(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	seen := map[string]bool{"ns/svc/s1!ip#1": true, "ns/svc/s2!ip#1": true}
	for i := 0; i < 3; i++ {
		ok, why := grants.Fits("ns", "n", "ns/svc/s3!ip#1", 3, 2, seen, now)
		assert.True(t, ok, why)
	}
}

// A reservation that already holds an address takes no new one: an object
// failing over to another node reserves the address it already has.
func TestAReservationThatHoldsAnAddressTakesNoNewOne(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	seen := map[string]bool{"ns/svc/s1!ip#1": true, "ns/svc/s2!ip#1": true, "ns/svc/s3!ip#1": true}
	ok, _ := grants.Fits("ns", "n", "ns/svc/s3!ip#1", 3, 3, seen, now)
	assert.True(t, ok, "the namespace is at its limit, and this address is one of the three")

	ok, _ = grants.Fits("ns", "n", "ns/svc/s4!ip#1", 3, 3, seen, now)
	assert.False(t, ok, "where a fourth one is not")
}

// A grant stops being counted once the cluster reports the address it was
// granted for, or the address would be counted twice.
func TestAGrantStopsCountingOnceItIsSeen(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	seen := map[string]bool{"ns/svc/s1!ip#1": true}
	ok, _ := grants.Fits("ns", "n", "ns/svc/s2!ip#1", 2, 1, seen, now)
	assert.True(t, ok)

	// The address came back around, and the namespace is at its limit for a
	// reason the cluster can see.
	seen["ns/svc/s2!ip#1"] = true
	ok, why := grants.Fits("ns", "n", "ns/svc/s3!ip#1", 2, 2, seen, now)
	assert.False(t, ok)
	assert.Contains(t, why, "already holds 2")
}

// A grant nothing ever reserved would ration the namespace for ever, so it is
// forgotten.
func TestAGrantNobodyReservedExpires(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	seen := map[string]bool{"ns/svc/s1!ip#1": true}
	ok, _ := grants.Fits("ns", "n", "ns/svc/s2!ip#1", 2, 1, seen, now)
	assert.True(t, ok)

	ok, _ = grants.Fits("ns", "n", "ns/svc/s3!ip#1", 2, 1, seen, now)
	assert.False(t, ok, "the grant is counted while it can still be reserved")

	ok, _ = grants.Fits("ns", "n", "ns/svc/s3!ip#1", 2, 1, seen, now.Add(31*time.Second))
	assert.True(t, ok, "and not after it expired")
}

// A claim is weighed within its own namespace and its own network.
func TestAGrantIsNotWeighedAgainstAnotherNamespaceOrNetwork(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	ok, _ := grants.Fits("ns", "n", "ns/svc/s1!ip#1", 1, 0, map[string]bool{}, now)
	assert.True(t, ok)

	ok, _ = grants.Fits("other", "n", "other/svc/s1!ip#1", 1, 0, map[string]bool{}, now)
	assert.True(t, ok, "another namespace holds its own claim")

	ok, _ = grants.Fits("ns", "other", "ns/svc/s2!ip#1", 1, 0, map[string]bool{}, now)
	assert.True(t, ok, "and another network is held addresses of separately")
}
