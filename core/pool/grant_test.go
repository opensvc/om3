package pool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	mi = int64(1024 * 1024)
)

// Three volumes of a namespace claiming 300mi of a pool capped at 350mi: one
// of them may grow by 50mi, and the two others may not, however stale the
// reading they are answered from.
func TestAGrantIsWeighedAgainstTheNextClaim(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{
		"ns/vol/v1": 100 * mi,
		"ns/vol/v2": 100 * mi,
		"ns/vol/v3": 100 * mi,
	}
	ok, _ := grants.Fits("ns", "p", "ns/vol/v1", 150*mi, 350*mi, held, now)
	assert.True(t, ok, "the first grow fits the claim")

	// The same reading, which is what a write that has not come back around
	// yet leaves every other claim to be answered from.
	ok, why := grants.Fits("ns", "p", "ns/vol/v2", 150*mi, 350*mi, held, now)
	assert.False(t, ok, "the second grow does not, and the first is why")
	assert.Contains(t, why, "already claims 350mi")

	ok, _ = grants.Fits("ns", "p", "ns/vol/v3", 150*mi, 350*mi, held, now)
	assert.False(t, ok)
}

// A claim answered yes and asked again is answered the same way: what the
// object is to hold is the question, not what it would add.
func TestAskingTwiceForTheSameSizeIsAnsweredTheSame(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{"ns/vol/v1": 100 * mi, "ns/vol/v2": 100 * mi}
	for i := 0; i < 3; i++ {
		ok, why := grants.Fits("ns", "p", "ns/vol/v1", 150*mi, 350*mi, held, now)
		assert.True(t, ok, why)
	}
}

// A grant stops being counted on its own once the configuration it was
// granted for says the same thing, or the volume would be counted twice.
func TestAGrantStopsCountingOnceItIsSeen(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{"ns/vol/v1": 100 * mi}
	ok, _ := grants.Fits("ns", "p", "ns/vol/v1", 300*mi, 350*mi, held, now)
	assert.True(t, ok)

	// The write came back around.
	held["ns/vol/v1"] = 300 * mi
	ok, _ = grants.Fits("ns", "p", "ns/vol/v2", 50*mi, 350*mi, held, now)
	assert.True(t, ok, "a second volume takes what is left, and the grant is not counted on top of the configuration")
}

// A grant nothing ever claimed would ration the namespace for ever, so it is
// forgotten.
func TestAGrantNobodyClaimedExpires(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{"ns/vol/v1": 100 * mi}
	ok, _ := grants.Fits("ns", "p", "ns/vol/v1", 350*mi, 350*mi, held, now)
	assert.True(t, ok)

	ok, _ = grants.Fits("ns", "p", "ns/vol/v2", 50*mi, 350*mi, held, now)
	assert.False(t, ok, "the grant is counted while it can still be claimed")

	ok, _ = grants.Fits("ns", "p", "ns/vol/v2", 50*mi, 350*mi, held, now.Add(31*time.Second))
	assert.True(t, ok, "and not after it expired")
}

// A claim is weighed within its own namespace and its own pool.
func TestAGrantIsNotWeighedAgainstAnotherNamespaceOrPool(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{"ns/vol/v1": 300 * mi}
	ok, _ := grants.Fits("ns", "p", "ns/vol/v1", 350*mi, 350*mi, held, now)
	assert.True(t, ok)

	ok, _ = grants.Fits("other", "p", "other/vol/v1", 350*mi, 350*mi, map[string]int64{}, now)
	assert.True(t, ok, "another namespace holds its own claim")

	ok, _ = grants.Fits("ns", "other", "ns/vol/v2", 350*mi, 350*mi, map[string]int64{}, now)
	assert.True(t, ok, "and another pool is claimed of separately")
}

// A volume with no name yet, which is one being created, has no configuration
// to report the claim, so its grant is kept on its own until it expires.
func TestGrantsOfUnnamedVolumesDoNotReplaceEachOther(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{}
	ok, _ := grants.Fits("ns", "p", "", 200*mi, 350*mi, held, now)
	assert.True(t, ok)
	ok, _ = grants.Fits("ns", "p", "", 200*mi, 350*mi, held, now)
	assert.False(t, ok, "the second allocation is weighed against the first")
}

// A lookup asking for no size is not an allocation, and leaves nothing
// behind to be forgotten later.
func TestAClaimOfNothingIsNotGranted(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{"ns/vol/v1": 100 * mi}
	ok, _ := grants.Fits("ns", "p", "", 0, 350*mi, held, now)
	assert.True(t, ok)
	ok, _ = grants.Fits("ns", "p", "ns/vol/v2", 250*mi, 350*mi, held, now)
	assert.True(t, ok, "and takes nothing from what is left")
}

// A node that has just taken the answering over has no grants of its own, and
// what was granted and written is on the node that wrote it. Seeding is how
// it is counted here before the configuration arrives.
func TestASeededGrantIsWeighedLikeOneAnswered(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{"ns/vol/v1": 100 * mi, "ns/vol/v2": 100 * mi}

	ok, _ := NewGrants(30*time.Second).Fits("ns", "p", "ns/vol/v3", 50*mi, 350*mi, held, now)
	assert.True(t, ok, "without the grant, a third volume fits what the cluster has been seen to hold")

	// The first volume was grown to 250mi on the node holding it, which this
	// one has not seen.
	grants.Seed("ns", "p", "ns/vol/v1", 250*mi, now)
	ok, why := grants.Fits("ns", "p", "ns/vol/v3", 50*mi, 350*mi, held, now)
	assert.False(t, ok, "with it, the namespace is already at its limit")
	assert.Contains(t, why, "already claims 350mi")

	// And it stops counting once the configuration says the same thing.
	held["ns/vol/v1"] = 250 * mi
	ok, _ = grants.Fits("ns", "p", "ns/vol/v3", 0, 350*mi, held, now)
	assert.True(t, ok)
	ok, _ = grants.Fits("ns", "p", "ns/vol/v3", 50*mi, 400*mi, held, now)
	assert.True(t, ok, "counted once, not twice")
}

// Rebuilding cannot lower what a namespace is counted as holding.
func TestASeedDoesNotLowerAGrant(t *testing.T) {
	grants := NewGrants(30 * time.Second)
	now := time.Now()
	held := map[string]int64{}
	ok, _ := grants.Fits("ns", "p", "ns/vol/v1", 300*mi, 350*mi, held, now)
	assert.True(t, ok)

	grants.Seed("ns", "p", "ns/vol/v1", 100*mi, now)
	ok, _ = grants.Fits("ns", "p", "ns/vol/v2", 100*mi, 350*mi, held, now)
	assert.False(t, ok, "the grant answered is what the volume was let take")
}
