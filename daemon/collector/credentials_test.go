package collector

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
)

// newCredsT returns the collector of n2, leader and first of the cluster
// nodes n2 n3, speaking through an instrumented requester. The collector of
// n3, alive, has the state peer3.
func newCredsT(t *testing.T, peer3 string) (*T, *instrumentedRequester) {
	node.InitData()
	daemonsubsystem.InitData()
	clusternode.Set([]string{"n2", "n3"})
	t.Cleanup(func() {
		clusternode.Set(nil)
		node.InitData()
		daemonsubsystem.InitData()
	})
	node.MonitorData.Set("n3", &node.Monitor{})
	c := daemonsubsystem.Collector{}
	c.State = peer3
	daemonsubsystem.DataCollector.Set("n3", &c)

	r := newInstrumentedRequester(&fakeRequester{})
	tr := newSpeakerT(t, nil)
	tr.client = r
	tr.localhost = "n2"
	tr.isSpeaker = false
	tr.nodeIsLeader = map[string]bool{"n2": true}
	tr.updateSpeaker()
	require.Equal(t, "n2", tr.speaker)
	return tr, r
}

// The credentials the collector answers 401 to for credsRefusedAfter are
// refused: the speech goes to the next node able to speak.
func TestRefusedCredentialsHandTheSpeechOver(t *testing.T) {
	tr, r := newCredsT(t, "speaker-candidate")
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)

	tr.checkCredentials(t0.Add(credsRefusedAfter - time.Second))
	assert.Equal(t, "n2", tr.speaker, "not refused before credsRefusedAfter")

	tr.checkCredentials(t0.Add(credsRefusedAfter))
	assert.True(t, tr.credsRefused())
	assert.Equal(t, credsRefusedMin, tr.credsRefusedBackoff)
	assert.Equal(t, "n3", tr.speaker)
	assert.False(t, tr.isSpeaker)
	assert.Equal(t, "warning", tr.getState())
}

// An answer not a 401 ends the run of 401: a collector refusing a moment
// does not move the speech.
func TestAnAcceptedRequestEndsTheRunOf401(t *testing.T) {
	tr, r := newCredsT(t, "speaker-candidate")
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)
	r.observe(http.StatusAccepted, t0.Add(20*time.Second))
	r.observe(http.StatusUnauthorized, t0.Add(25*time.Second))

	tr.checkCredentials(t0.Add(40 * time.Second))
	assert.False(t, tr.credsRefused())
	assert.Equal(t, "n2", tr.speaker)
}

// An answer neither accepted nor 401, as a 403 or a 5xx, tells nothing of the
// credentials: it neither ends the run of 401 nor counts as an acceptance.
func TestAnErrorAnswerTellsNothingOfTheCredentials(t *testing.T) {
	tr, r := newCredsT(t, "speaker-candidate")
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)
	r.observe(http.StatusInternalServerError, t0.Add(20*time.Second))
	r.observe(http.StatusForbidden, t0.Add(30*time.Second))
	assert.True(t, tr.acceptedAt().IsZero())

	tr.checkCredentials(t0.Add(credsRefusedAfter))
	assert.True(t, tr.credsRefused(), "the run of 401 went on through the errors")

	tr.checkCredentials(tr.credsRefusedUntil)
	offered := tr.credsOfferedAt
	r.observe(http.StatusBadGateway, offered.Add(time.Second))
	tr.checkCredentials(offered.Add(2 * time.Second))
	assert.Equal(t, credsRefusedMin, tr.credsRefusedBackoff, "a 502 is no acceptance: the backoff stays")
}

// A new node.uuid is offered at once, and the backoff forgotten.
func TestANewNodeUUIDIsOfferedAtOnce(t *testing.T) {
	tr, r := newCredsT(t, "speaker-candidate")
	tr.appliedConfig = &collector.Config{FeederUrl: "https://collector.example.com", Password: "uuid-old"}
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)
	tr.checkCredentials(t0.Add(credsRefusedAfter))
	require.Equal(t, "n3", tr.speaker)

	tr.applyConfig(&collector.Config{FeederUrl: "https://collector.example.com", Password: "uuid-old"})
	assert.True(t, tr.credsRefused(), "the same node.uuid stays refused")

	tr.applyConfig(&collector.Config{FeederUrl: "https://collector.example.com", Password: "uuid-new"})
	assert.False(t, tr.credsRefused())
	assert.Zero(t, tr.credsRefusedBackoff)
	assert.Equal(t, "n2", tr.speaker)
}

// Another collector is offered the credentials at once: the refusal of the
// former one tells nothing of it.
func TestAnotherCollectorIsOfferedAtOnce(t *testing.T) {
	tr, r := newCredsT(t, "speaker-candidate")
	tr.appliedConfig = &collector.Config{FeederUrl: "https://collector.example.com", Password: "uuid-old"}
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)
	tr.checkCredentials(t0.Add(credsRefusedAfter))
	require.True(t, tr.credsRefused())

	tr.applyConfig(&collector.Config{FeederUrl: "https://other.example.com", Password: "uuid-old"})
	assert.False(t, tr.credsRefused())
	assert.Equal(t, "n2", tr.speaker)
}

// Refused credentials are offered again at the end of the backoff, which
// doubles at each refusal, and is forgotten once the collector accepts them.
func TestRefusedCredentialsAreOfferedAgainAfterTheBackoff(t *testing.T) {
	tr, r := newCredsT(t, "speaker-candidate")
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)
	tr.checkCredentials(t0.Add(credsRefusedAfter))
	require.True(t, tr.credsRefused())

	t1 := tr.credsRefusedUntil
	tr.checkCredentials(t1)
	assert.False(t, tr.credsRefused(), "offered again")
	assert.Equal(t, "n2", tr.speaker)

	r.observe(http.StatusUnauthorized, t1.Add(time.Second))
	tr.checkCredentials(t1.Add(time.Second + credsRefusedAfter))
	assert.True(t, tr.credsRefused(), "refused again")
	assert.Equal(t, 2*credsRefusedMin, tr.credsRefusedBackoff)

	t2 := tr.credsRefusedUntil
	tr.checkCredentials(t2)
	r.observe(http.StatusAccepted, t2.Add(time.Second))
	tr.checkCredentials(t2.Add(2 * time.Second))
	assert.False(t, tr.credsRefused())
	assert.Zero(t, tr.credsRefusedBackoff, "accepted again")
}

// With no other node able to speak, the leader keeps the speech, its state
// saying the warning.
func TestEveryNodeRefusedTheLeaderSpeaks(t *testing.T) {
	tr, r := newCredsT(t, "warning")
	t0 := time.Now()
	r.observe(http.StatusUnauthorized, t0)
	tr.checkCredentials(t0.Add(credsRefusedAfter))
	assert.True(t, tr.credsRefused())
	assert.Equal(t, "n2", tr.speaker)
	assert.True(t, tr.isSpeaker)
	assert.Equal(t, "speaker-warning", tr.getState())
}
