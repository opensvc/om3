package prefetch

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

var (
	nginx    = resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}
	pause    = resource.Pull{Engine: "docker", Image: "ghcr.io/opensvc/pause:latest"}
	podNginx = resource.Pull{Engine: "podman", Image: "docker.io/library/nginx:latest"}
	rootless = resource.Pull{Engine: "podman", Store: "app", Image: "docker.io/library/nginx:latest"}
)

// world is the node of a test: the stores of its engines, its registry,
// its clock, and what the manager did there.
type world struct {
	t *testing.T
	m *Manager

	clock time.Time

	// recorded counts the outcomes the commands recorded, each stamped
	// later than the one before, as the clock of a node stamps them.
	recorded int

	// present are the images the stores hold, with their id.
	present map[resource.Pull]string

	// unreachable are the images the registry does not serve, with why.
	unreachable map[resource.Pull]string

	// engineDown says the engines do not answer.
	engineDown bool

	// checks and execs are the questions asked to the stores, and the
	// commands run, in their order.
	checks []resource.Pull
	execs  []string

	jobs []job

	mu   sync.Mutex
	msgs []pubsub.Messager
}

func (w *world) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.msgs = append(w.msgs, m)
}

// prefetchUpdates returns the outcome changes published, as "state image",
// and forgets them.
func (w *world) prefetchUpdates() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	l := make([]string, 0)
	for _, m := range w.msgs {
		if c, ok := m.(*msgbus.ImagePrefetchUpdated); ok {
			l = append(l, c.State+" "+c.Pull.Image)
		}
	}
	w.msgs = nil
	return l
}

func newWorld(t *testing.T) *world {
	t.Helper()
	testhelper.Setup(t)
	w := &world{
		t:           t,
		clock:       time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC),
		present:     make(map[resource.Pull]string),
		unreachable: make(map[resource.Pull]string),
	}
	m := NewManager(nil)
	m.log = plog.NewDefaultLogger()
	m.ctx = context.Background()
	m.localhost = "n1"
	m.publisher = w
	m.now = func() time.Time { return w.clock }
	m.jitter = func(time.Duration) time.Duration { return 0 }
	m.check = func(_ context.Context, pull resource.Pull) (bool, string, bool, error) {
		w.checks = append(w.checks, pull)
		if pull.Store != "" {
			return false, "", false, nil
		}
		if w.engineDown {
			return false, "", true, errors.New("the engine does not answer")
		}
		id, ok := w.present[pull]
		return ok, id, true, nil
	}
	// The command of an object does what the driver does: it looks in
	// the store, pulls a missing image, and records the outcome.
	m.exec = func(_ context.Context, p naming.Path, rid string) error {
		w.execs = append(w.execs, p.String()+" "+rid)
		pull, ok := m.objects[p][rid]
		if !ok {
			return errors.New("no such resource")
		}
		if w.engineDown {
			return errors.New("the engine does not answer")
		}
		w.recorded++
		outcome := resource.PrefetchOutcome{Key: pull, UpdatedAt: w.clock.Add(time.Duration(w.recorded) * time.Millisecond)}
		if id, ok := w.present[pull]; ok {
			outcome.State = resource.PrefetchPresent
			outcome.ImageID = id
		} else if why, ok := w.unreachable[pull]; ok {
			outcome.State = resource.PrefetchFailed
			outcome.Error = why
		} else {
			w.present[pull] = "sha256:pulled"
			outcome.State = resource.PrefetchPulled
			outcome.Source = resource.PrefetchSourceRegistry
			outcome.ImageID = "sha256:pulled"
		}
		return outcome.Write()
	}
	m.start = func(j job) { w.jobs = append(w.jobs, j) }
	w.m = m
	return w
}

// run runs the jobs due at the clock of the node, one after the other,
// until none is.
func (w *world) run() {
	w.t.Helper()
	for range 100 {
		w.m.tick()
		if len(w.jobs) == 0 {
			return
		}
		j := w.jobs[0]
		w.jobs = nil
		w.m.onResult(w.m.do(context.Background(), j))
	}
	w.t.Fatal("the jobs never end")
}

// after moves the clock of the node, and runs the jobs due then.
func (w *world) after(d time.Duration) {
	w.t.Helper()
	w.clock = w.clock.Add(d)
	w.run()
}

func path(t *testing.T, s string) naming.Path {
	t.Helper()
	p, err := naming.ParsePath(s)
	require.NoError(t, err)
	return p
}

// config publishes the instance config of an object of the node, whose
// resources pull the images given by rid.
func (w *world) config(p string, pulls map[string]resource.Pull) {
	w.t.Helper()
	resources := make(instance.ResourceConfigs)
	for rid, pull := range pulls {
		resources[rid] = instance.ResourceConfig{Pull: &pull}
	}
	resources["fs#1"] = instance.ResourceConfig{}
	w.m.onEvent(&msgbus.InstanceConfigUpdated{
		Path:  path(w.t, p),
		Node:  "n1",
		Value: instance.Config{ActorConfig: &instance.ActorConfig{Resources: resources}},
	})
}

func (w *world) removed(driverType string) {
	w.m.onEvent(&msgbus.ContainerImageRemoved{Node: "n1", Type: driverType})
}

func outcomeState(pull resource.Pull) string {
	outcome, ok := resource.ReadPrefetchOutcome(pull)
	if !ok {
		return ""
	}
	return outcome.State
}

// The resources pulling the same image, whatever their objects, are one
// key: it is pulled once, by the command of the first object, and an image
// the store holds is not pulled at all.
func TestOnePullPerKey(t *testing.T) {
	w := newWorld(t)
	w.present[pause] = "sha256:pause"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": nginx, "container#3": pause})
	w.config("ns1/svc/api", map[string]resource.Pull{"container#1": nginx})
	w.config("ns2/svc/web", map[string]resource.Pull{"container#9": nginx})
	w.run()
	assert.Equal(t, []string{"ns1/svc/api container#1"}, w.execs, "one pull, by the first object of the key")
	assert.Len(t, w.checks, 2, "each key is asked to its store once")
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))
	assert.Equal(t, "", outcomeState(pause), "an image found in the store by the daemon records nothing")
	assert.Empty(t, w.prefetchUpdates(), "nothing failed")

	// Settled, nothing is asked again, however long.
	w.after(24 * time.Hour)
	assert.Len(t, w.checks, 2)
	assert.Len(t, w.execs, 1)
}

// The first check of a key waits a delay drawn at random, for the nodes
// receiving the same configuration not to pull together.
func TestANewKeyIsCheckedAfterADelayDrawnAtRandom(t *testing.T) {
	w := newWorld(t)
	w.m.jitter = func(d time.Duration) time.Duration {
		assert.Equal(t, initialDelay, d)
		return 40 * time.Second
	}
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.after(39 * time.Second)
	assert.Empty(t, w.checks)
	assert.Equal(t, w.clock.Add(time.Second), w.m.nextWake())
	w.after(time.Second)
	assert.Equal(t, []string{"ns1/svc/web container#1"}, w.execs)
	assert.True(t, w.m.nextWake().Equal(w.m.sweepAt), "nothing but the sweep to wake for")
}

// A pull that failed is tried again later, less and less often, the
// instances pulling the image told when it fails and when it stops
// failing.
func TestAFailedPullIsTriedAgainWithABackoff(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "registry unreachable"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	assert.Len(t, w.execs, 1)
	assert.Equal(t, resource.PrefetchFailed, outcomeState(nginx))
	assert.Equal(t, []string{"failed docker.io/library/nginx:latest"}, w.prefetchUpdates())

	// 5m, 10m, 20m, ... up to 4h.
	tries := 1
	for _, delay := range []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, 80 * time.Minute, 160 * time.Minute, 4 * time.Hour, 4 * time.Hour} {
		w.after(delay - time.Second)
		assert.Len(t, w.execs, tries, "not before %s", delay)
		w.after(time.Second)
		tries++
		assert.Len(t, w.execs, tries, "after %s", delay)
	}
	assert.Empty(t, w.prefetchUpdates(), "failing still is no news")

	// The registry back, the next try pulls, and the backoff is over.
	delete(w.unreachable, nginx)
	w.after(4 * time.Hour)
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))
	assert.Equal(t, []string{"pulled docker.io/library/nginx:latest"}, w.prefetchUpdates())
	assert.Zero(t, w.m.keys[nginx].attempts)
	tries = len(w.execs)
	w.after(24 * time.Hour)
	assert.Len(t, w.execs, tries)
}

// The retry delay is drawn longer by up to a fifth, for the nodes failing
// together not to try together.
func TestTheRetryDelayIsDrawnLonger(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "registry unreachable"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.m.jitter = func(d time.Duration) time.Duration { return d / 2 }
	w.after(initialDelay / 2)
	require.Len(t, w.execs, 1)
	assert.Equal(t, w.clock.Add(5*time.Minute+30*time.Second), w.m.keys[nginx].dueAt)
}

// A change of the configuration of an object whose key failed may be the
// fix: the key is tried again at once, its backoff over.
func TestAConfigChangeTriesAFailedKeyAgainAtOnce(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "pull access denied"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	w.after(5 * time.Minute)
	w.after(10 * time.Minute)
	require.Len(t, w.execs, 3)

	delete(w.unreachable, nginx)
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	assert.Len(t, w.execs, 4, "tried at once")
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))

	// A change of an object whose keys are settled asks nothing.
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	assert.Len(t, w.execs, 4)
}

// Nothing is known of the image of an engine that does not answer: no
// outcome is recorded, nobody is told, and the key is tried again later.
func TestAnEngineThatDoesNotAnswerIsTriedAgain(t *testing.T) {
	w := newWorld(t)
	w.engineDown = true
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": rootless})
	w.run()
	assert.Equal(t, []string{"ns1/svc/web container#2"}, w.execs, "the store of the user is the command's to ask")
	assert.Equal(t, "", outcomeState(nginx))
	assert.Equal(t, "", outcomeState(rootless))
	assert.Empty(t, w.prefetchUpdates())
	assert.Equal(t, 1, w.m.keys[nginx].attempts)

	w.engineDown = false
	w.after(5 * time.Minute)
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))
	assert.Equal(t, resource.PrefetchPulled, outcomeState(rootless))
	assert.Empty(t, w.prefetchUpdates(), "nothing was known failed")
}

// The store of a user is not one the daemon reads: its key goes through
// the command of the object, which looks there as that user.
func TestAKeyOfAStoreTheDaemonDoesNotReadGoesThroughTheCommand(t *testing.T) {
	w := newWorld(t)
	w.present[rootless] = "sha256:there"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": rootless})
	w.run()
	assert.Equal(t, []string{"ns1/svc/web container#1"}, w.execs)
	assert.Equal(t, resource.PrefetchPresent, outcomeState(rootless))
	assert.Equal(t, "sha256:there", w.m.keys[rootless].imageID)
}

// An image removed from the store of an engine has the settled keys of that
// store checked again, once for a burst of removals, and the missing ones
// pulled again: the keys of another engine, and of the store of a user,
// which no event tells the removals of, are not.
func TestAnImageRemovalChecksTheKeysOfItsEngineAgain(t *testing.T) {
	w := newWorld(t)
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": pause, "container#3": podNginx, "container#4": rootless})
	w.run()
	require.Len(t, w.execs, 4)
	w.checks = nil

	// A prune removed nginx, telling the id of each image it removed,
	// which docker names no other way.
	delete(w.present, nginx)
	for range 20 {
		w.m.onEvent(&msgbus.ContainerImageRemoved{Node: "n1", Type: "container.docker", ID: "sha256:abc"})
	}
	w.run()
	assert.Empty(t, w.checks, "the removals settle first")
	w.after(recheckDelay)
	assert.ElementsMatch(t, []resource.Pull{nginx, pause}, w.checks, "each docker key once")
	assert.Len(t, w.execs, 5, "the missing one is pulled again")
	assert.Equal(t, "ns1/svc/web container#1", w.execs[4])
	assert.Contains(t, w.present, nginx)

	// A stream that started again names no image.
	w.checks = nil
	w.removed("container.podman")
	w.after(recheckDelay)
	assert.Equal(t, []resource.Pull{podNginx}, w.checks)
	assert.Len(t, w.execs, 5)

	// The removals of another node are not the ones of this store.
	w.checks = nil
	w.m.onEvent(&msgbus.ContainerImageRemoved{Node: "n2", Type: "container.docker"})
	w.after(recheckDelay)
	assert.Empty(t, w.checks)
}

// The outcome a former prefetch left in the file of a key is not the one of
// a command that recorded none for it, as the command run for a resource
// whose configuration names another image since: the command pulled the
// other image, the one of the key is still missing, and the key is tried
// again instead of settled on what the file says.
func TestAnOutcomeLeftByAFormerPrefetchIsNotTheOneOfTheCommand(t *testing.T) {
	w := newWorld(t)
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.config("ns2/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	require.Equal(t, []string{"ns1/svc/web container#1"}, w.execs)
	require.Equal(t, resource.PrefetchPulled, outcomeState(nginx))

	// A prune removed nginx, and the configuration of the resource the
	// key is pulled by names pause, which the daemon is not told yet.
	delete(w.present, nginx)
	w.removed("container.docker")
	exec := w.m.exec
	w.m.exec = func(ctx context.Context, p naming.Path, rid string) error {
		w.m.objects[p][rid] = pause
		defer func() { w.m.objects[p][rid] = nginx }()
		return exec(ctx, p, rid)
	}
	w.after(recheckDelay)
	require.Len(t, w.execs, 2)
	require.Equal(t, resource.PrefetchPulled, outcomeState(pause), "the command fetched the other image")
	assert.NotContains(t, w.present, nginx)

	e := w.m.keys[nginx]
	assert.Equal(t, 1, e.attempts)
	assert.Equal(t, w.clock.Add(5*time.Minute), e.dueAt, "tried again, with the backoff of a failure")
	assert.Equal(t, resource.PrefetchPulled, e.state, "nothing is known of the image: the state stays")
	assert.Empty(t, w.prefetchUpdates())

	// The command of the next try records the outcome of the key.
	w.m.exec = exec
	w.after(5 * time.Minute)
	require.Len(t, w.execs, 3)
	assert.Contains(t, w.present, nginx)
	assert.Equal(t, 0, e.attempts)
	assert.True(t, e.dueAt.IsZero())
}

// A removal does not hurry a key waiting for its next try: its backoff
// stands.
func TestAnImageRemovalDoesNotHurryAFailedKey(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "registry unreachable"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	require.Len(t, w.execs, 1)
	w.removed("container.docker")
	w.after(recheckDelay)
	assert.Len(t, w.execs, 1)
	w.after(5 * time.Minute)
	assert.Len(t, w.execs, 2)
}

// An image found in the store after its prefetch failed, as one a start
// pulled, is no failure any more: the outcome is forgotten, and the
// instances told.
func TestAnImagePresentAfterAFailedPrefetchClearsTheFailure(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "registry unreachable"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	require.Equal(t, []string{"failed docker.io/library/nginx:latest"}, w.prefetchUpdates())

	w.present[nginx] = "sha256:started"
	w.after(5 * time.Minute)
	assert.Len(t, w.execs, 1, "nothing to pull")
	assert.Equal(t, "", outcomeState(nginx))
	assert.Equal(t, []string{"present docker.io/library/nginx:latest"}, w.prefetchUpdates())
	assert.True(t, w.m.keys[nginx].dueAt.IsZero())
}

// A key no resource pulls any more is dropped with its outcome, when its
// last resource names another image, disables its prefetch, or goes with
// its object. A key another object still pulls stays.
func TestAKeyNoResourcePullsIsDroppedWithItsOutcome(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "registry unreachable"
	w.unreachable[pause] = "registry unreachable"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": pause})
	w.config("ns1/svc/api", map[string]resource.Pull{"container#1": nginx})
	w.run()
	require.Equal(t, resource.PrefetchFailed, outcomeState(nginx))
	require.Equal(t, resource.PrefetchFailed, outcomeState(pause))

	// web names no image to prefetch any more: pause goes, nginx stays
	// for api.
	w.config("ns1/svc/web", nil)
	assert.Equal(t, "", outcomeState(pause))
	assert.Equal(t, resource.PrefetchFailed, outcomeState(nginx))
	assert.NotContains(t, w.m.keys, pause)
	require.Contains(t, w.m.keys, nginx)
	p, rid := w.m.keys[nginx].target()
	assert.Equal(t, "ns1/svc/api container#1", p.String()+" "+rid)

	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: path(t, "ns1/svc/api"), Node: "n1"})
	assert.Equal(t, "", outcomeState(nginx))
	assert.Empty(t, w.m.keys)
	assert.Empty(t, w.m.objects)
	w.after(24 * time.Hour)
	assert.Len(t, w.execs, 2, "nothing is tried for a key dropped")
}

// A key dropped while its job runs is not followed, and the outcome the
// command recorded meanwhile is removed.
func TestAKeyDroppedWhileItsJobRuns(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "registry unreachable"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.m.tick()
	require.Len(t, w.jobs, 1)
	r := w.m.do(context.Background(), w.jobs[0])
	w.jobs = nil
	require.Equal(t, resource.PrefetchFailed, outcomeState(nginx))
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: path(t, "ns1/svc/web"), Node: "n1"})
	w.m.onResult(r)
	assert.Equal(t, "", outcomeState(nginx))
	assert.Empty(t, w.prefetchUpdates())
	assert.Nil(t, w.m.running)
}

// peerStatus publishes the instance status of an object on a node, with the
// resources given up, and the others it has down.
func (w *world) peerStatus(p, node string, up ...string) {
	w.t.Helper()
	resources := instance.ResourceStatuses{
		"container#1": {Status: status.Down},
		"container#2": {Status: status.Down},
		"fs#1":        {Status: status.Down},
	}
	for _, rid := range up {
		resources[rid] = resource.Status{Status: status.Up}
	}
	w.m.onEvent(&msgbus.InstanceStatusUpdated{
		Path:  path(w.t, p),
		Node:  node,
		Value: instance.Status{Resources: resources},
	})
}

// A container of an object turning up on another node is what follows an
// image loaded there by hand: the keys of the object that failed are tried
// again at once, all of them, the node now holding an image to copy. The
// container staying up asks nothing more, and one turning up again does.
func TestAContainerTurningUpOnAPeerTriesTheFailedKeysOfItsObjectAgainAtOnce(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "no registry"
	w.unreachable[pause] = "no registry"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": pause})
	w.peerStatus("ns1/svc/web", "n2")
	w.run()
	w.after(5 * time.Minute)
	require.Len(t, w.execs, 4, "both keys failed twice, and wait for 10 minutes")

	// The image was loaded on n2, where the object is started: the
	// command of the object now copies it from there.
	delete(w.unreachable, nginx)
	w.peerStatus("ns1/svc/web", "n2", "container#1")
	w.run()
	assert.Len(t, w.execs, 6, "both keys are tried at once, a container telling of its object")
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))
	assert.Equal(t, resource.PrefetchFailed, outcomeState(pause))
	assert.ElementsMatch(t, []string{"failed docker.io/library/nginx:latest", "failed ghcr.io/opensvc/pause:latest", "pulled docker.io/library/nginx:latest"}, w.prefetchUpdates())

	// The backoff of the key still failing started again.
	w.after(5 * time.Minute)
	assert.Len(t, w.execs, 7)

	// Up still, as each status of the instance says: nothing new.
	w.peerStatus("ns1/svc/web", "n2", "container#1")
	w.run()
	assert.Len(t, w.execs, 7)

	// Up on another node, then down and up again on the first.
	w.peerStatus("ns1/svc/web", "n3", "container#1")
	w.run()
	assert.Len(t, w.execs, 8)
	w.peerStatus("ns1/svc/web", "n2")
	w.run()
	assert.Len(t, w.execs, 8, "a container going down tells of no image")
	w.peerStatus("ns1/svc/web", "n2", "container#1")
	w.run()
	assert.Len(t, w.execs, 9)

	// An instance deleted and made again turns its containers up again.
	w.m.onEvent(&msgbus.InstanceStatusDeleted{Path: path(t, "ns1/svc/web"), Node: "n2"})
	w.peerStatus("ns1/svc/web", "n2", "container#1")
	w.run()
	assert.Len(t, w.execs, 10)
}

// What turns up elsewhere tells of the keys of its object alone, when a
// resource pulling an image does: not a resource pulling none, not the
// instance of this node, not an object this node does not have, and not a
// key that did not fail.
func TestWhatTurnsUpOnAPeerTriesNoOtherKey(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "no registry"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.config("ns1/svc/db", map[string]resource.Pull{"container#1": pause})
	w.run()
	require.ElementsMatch(t, []string{"ns1/svc/web container#1", "ns1/svc/db container#1"}, w.execs)
	w.execs = nil

	w.peerStatus("ns1/svc/web", "n2", "fs#1")
	w.peerStatus("ns1/svc/web", "n1", "container#1")
	w.peerStatus("ns1/svc/db", "n2", "container#1")
	w.peerStatus("ns1/svc/other", "n2", "container#1")
	w.peerStatus("ns2/svc/web", "n2", "container#1")
	w.run()
	assert.Empty(t, w.execs)
	assert.Empty(t, w.checks[2:], "the settled key of the object whose container is up is not checked again")

	w.peerStatus("ns1/svc/web", "n2", "container#1")
	w.run()
	assert.Equal(t, []string{"ns1/svc/web container#1"}, w.execs)
}

// A container turning up while the command of its key runs is tried again
// after it: the command may have asked the node before the image was there.
func TestAContainerTurningUpOnAPeerWhileTheKeyIsFetched(t *testing.T) {
	w := newWorld(t)
	w.unreachable[nginx] = "no registry"
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	w.run()
	require.Len(t, w.execs, 1)

	w.clock = w.clock.Add(5 * time.Minute)
	w.m.tick()
	require.Len(t, w.jobs, 1)
	r := w.m.do(context.Background(), w.jobs[0])
	w.jobs = nil
	require.Len(t, w.execs, 2, "the command failed, and its result is not read yet")
	delete(w.unreachable, nginx)
	w.peerStatus("ns1/svc/web", "n2", "container#1")
	w.m.onResult(r)
	w.run()
	assert.Len(t, w.execs, 3, "tried again at once, not after the backoff")
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))
}

// The containers up on the other nodes of an object are forgotten with the
// object, and with the resources it no longer has.
func TestThePeersOfAnObjectAreForgottenWithIt(t *testing.T) {
	w := newWorld(t)
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": pause})
	w.peerStatus("ns1/svc/web", "n2", "container#1", "container#2")
	require.Len(t, w.m.peerUp[path(t, "ns1/svc/web")], 2)
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	assert.Len(t, w.m.peerUp[path(t, "ns1/svc/web")], 1)
	w.m.onEvent(&msgbus.InstanceConfigDeleted{Path: path(t, "ns1/svc/web"), Node: "n1"})
	assert.Empty(t, w.m.peerUp)
}

// One job runs at a time: a key due while another is pulled waits for it.
func TestOneJobAtATime(t *testing.T) {
	w := newWorld(t)
	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx, "container#2": pause})
	w.m.tick()
	w.m.tick()
	require.Len(t, w.jobs, 1)
	assert.True(t, w.m.nextWake().IsZero(), "the result of the job is waited for, not the clock")
	first := w.jobs[0]
	w.jobs = nil
	w.m.onResult(w.m.do(context.Background(), first))
	w.m.tick()
	require.Len(t, w.jobs, 1)
	assert.NotEqual(t, first.pull, w.jobs[0].pull)
}

// A daemon starting knows the failures recorded before it stopped, and
// removes, a while after, the outcomes of keys the node no longer has.
func TestAStartingDaemonSweepsTheOutcomesOfNoKey(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, resource.PrefetchOutcome{Key: nginx, State: resource.PrefetchFailed, Error: "registry unreachable"}.Write())
	require.NoError(t, resource.PrefetchOutcome{Key: pause, State: resource.PrefetchFailed}.Write())
	stray := resource.PrefetchDir() + "/docker/stray.json"
	require.NoError(t, os.WriteFile(stray, []byte("{"), 0o644))
	w.m.sweepAt = w.clock.Add(sweepDelay)

	w.config("ns1/svc/web", map[string]resource.Pull{"container#1": nginx})
	assert.Equal(t, resource.PrefetchFailed, w.m.keys[nginx].state)
	w.run()
	assert.Equal(t, []string{"pulled docker.io/library/nginx:latest"}, w.prefetchUpdates(), "the failure known from the file is over")
	assert.Equal(t, resource.PrefetchFailed, outcomeState(pause), "not swept yet: the objects may not all have spoken")

	w.after(sweepDelay)
	assert.Equal(t, "", outcomeState(pause))
	assert.NoFileExists(t, stray)
	assert.Equal(t, resource.PrefetchPulled, outcomeState(nginx))
	assert.True(t, w.m.sweepAt.IsZero(), "once")
}

// The command the daemon runs is cut short when the daemon stops.
func TestStopEndsTheRunningCommand(t *testing.T) {
	testhelper.Setup(t)
	bus := pubsub.NewBus(t.Name())
	bus.Start(context.Background())
	defer bus.Stop()
	ctx := pubsub.ContextWithBus(context.Background(), bus)

	m := NewManager(pubsub.WithQueueSize(100))
	m.jitter = func(time.Duration) time.Duration { return 0 }
	m.check = func(context.Context, resource.Pull) (bool, string, bool, error) { return false, "", true, nil }
	started := make(chan struct{})
	ended := make(chan error, 1)
	m.exec = func(ctx context.Context, _ naming.Path, _ string) error {
		close(started)
		<-ctx.Done()
		ended <- ctx.Err()
		return ctx.Err()
	}
	require.NoError(t, m.Start(ctx))
	pull := nginx
	pubsub.PubFromContext(ctx).Pub(&msgbus.InstanceConfigUpdated{
		Path: path(t, "ns1/svc/web"),
		Node: m.localhost,
		Value: instance.Config{ActorConfig: &instance.ActorConfig{Resources: instance.ResourceConfigs{
			"container#1": {Pull: &pull},
		}}},
	}, m.labelLocalhost)
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the command was not run")
	}
	stopped := make(chan struct{})
	go func() {
		_ = m.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("the stop waits for the command")
	}
	assert.ErrorIs(t, <-ended, context.Canceled)
}

// The command the daemon runs is announced on the bus, with its end: a
// command that failed is an error saying why, as its last "Error: " line
// does, and one that ran is none, whatever the pull did.
func TestExecCommand(t *testing.T) {
	om := func(script string) string {
		p := t.TempDir() + "/om"
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755))
		return p
	}
	kinds := func(w *world) []string {
		w.mu.Lock()
		defer w.mu.Unlock()
		l := make([]string, 0)
		for _, m := range w.msgs {
			l = append(l, m.(interface{ Kind() string }).Kind())
		}
		w.msgs = nil
		return l
	}
	w := newWorld(t)
	p := path(t, "ns1/svc/web")

	w.m.om = om(`test "$*" = "ns1/svc/web instance prefetch --rid container#1" || exit 3; test "$OSVC_ACTION_ORIGIN" = daemon/prefetch || exit 4`)
	require.NoError(t, w.m.execCommand(context.Background(), p, "container#1"))
	assert.Equal(t, []string{"Exec", "ExecSuccess"}, kinds(w))

	w.m.om = om(`echo "Error: ns1/svc/web: not found" >&2; exit 1`)
	err := w.m.execCommand(context.Background(), p, "container#1")
	require.ErrorContains(t, err, "ns1/svc/web: not found")
	assert.Equal(t, []string{"Exec", "ExecFailed"}, kinds(w))

	w.m.om = t.TempDir() + "/none"
	require.Error(t, w.m.execCommand(context.Background(), p, "container#1"))
	assert.Equal(t, []string{"Exec", "ExecFailed"}, kinds(w), "an exec announced is ended, even one that never ran")
}

// The words of the command name the object and its resource.
func TestCmdArgs(t *testing.T) {
	assert.Equal(t, []string{"ns1/svc/web", "instance", "prefetch", "--rid", "container#1"}, CmdArgs(path(t, "ns1/svc/web"), "container#1"))
}
