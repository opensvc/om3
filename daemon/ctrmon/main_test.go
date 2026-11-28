package ctrmon

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/evwatch"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// The lines podman 4.9.3 prints, captured on a dev cluster node for a
// container run, paused, unpaused, killed and removed. The state changes are
// read, the steps of a start or a stop that change nothing of it are not.
func TestParsePodmanEvent(t *testing.T) {
	line := func(status string) string {
		return `{"ID":"c3be1ef82f46","Image":"docker.io/library/alpine:3","Name":"omtest-evt-p","Status":"` + status + `","Time":"2026-11-13T18:30:54.025578777+01:00","Type":"container","Attributes":null}`
	}
	for status, want := range map[string]bool{
		"create": false, "init": false, "start": true, "pause": true, "unpause": true,
		"kill": false, "died": true, "cleanup": false, "remove": true, "exec_died": false,
		"health_status": true,
	} {
		name, ok := parsePodmanEvent(line(status))
		assert.Equal(t, want, ok, status)
		if ok {
			assert.Equal(t, "omtest-evt-p", name)
		}
	}
	_, ok := parsePodmanEvent(`{"ContainerExitCode":137,"ID":"c3be","Name":"x","Status":"died","Type":"image"}`)
	assert.False(t, ok, "not a container")
	_, ok = parsePodmanEvent("Error: unable to connect")
	assert.False(t, ok)
}

// The lines docker 29.1.3 prints, captured on a dev cluster node for a
// container run, stopped and removed.
func TestParseDockerEvent(t *testing.T) {
	line := func(action string) string {
		return `{"Type":"container","Action":"` + action + `","Actor":{"ID":"56bccc0e5817","Attributes":{"image":"nginx:1.30-alpine","name":"omtest-evt-d"}},"scope":"local","time":1794591054,"timeNano":1794591054000000000}`
	}
	for action, want := range map[string]bool{
		"create": false, "start": true, "kill": false, "stop": true, "die": true, "destroy": true,
		"exec_start: sh -c true": false, "health_status: healthy": true, "oom": true,
	} {
		name, ok := parseDockerEvent(line(action))
		assert.Equal(t, want, ok, action)
		if ok {
			assert.Equal(t, "omtest-evt-d", name)
		}
	}
	_, ok := parseDockerEvent(`{"Type":"network","Action":"connect","Actor":{"Attributes":{"name":"bridge"}}}`)
	assert.False(t, ok, "not a container")
}

// The lines podman 4.9.3 prints, captured on a dev cluster node for the rmi
// of a tag, the rmi of the last tag of an image, a prune and an rmi by id:
// an image untagged or removed is read, with its name when the line has
// one, and the events of an image pulled or tagged are not.
func TestParsePodmanImageEvent(t *testing.T) {
	const id = "e2ac70e7319a02c5a477f5825259bd118b94e8b02c279c67afa63adab6d8685b"
	for line, want := range map[string]*imageRemoval{
		`{"ID":"` + id + `","Name":"docker.io/library/hello-world:linux","Status":"pull","Time":"2026-11-27T09:19:43.323944404+01:00","Type":"image","Attributes":null}`:                                        nil,
		`{"ID":"` + id + `","Name":"prefetch-probe:t1","Status":"tag","Time":"2026-11-27T09:19:43.375529841+01:00","Type":"image","Attributes":null}`:                                                           nil,
		`{"ID":"` + id + `","Name":"localhost/prefetch-probe:t1","Status":"untag","Time":"2026-11-27T09:19:43.413558671+01:00","Type":"image","Attributes":null}`:                                               {name: "localhost/prefetch-probe:t1", id: id},
		`{"ID":"` + id + `","Name":"docker.io/library/hello-world:linux","Status":"remove","Time":"2026-11-27T09:19:43.447694786+01:00","Type":"image","Attributes":null}`:                                      {name: "docker.io/library/hello-world:linux", id: id},
		`{"ID":"` + id + `","Status":"remove","Time":"2026-11-27T09:19:43.641610976+01:00","Type":"image","Attributes":null}`:                                                                                   {id: id},
		`{"ID":"` + id + `","Name":"e2ac70e7319a","Status":"remove","Time":"2026-11-27T09:19:43.871376749+01:00","Type":"image","Attributes":null}`:                                                             {id: id},
		`{"ID":"52fb2cff203e","Image":"docker.io/library/busybox:latest","Name":"prefetch-probe-ctr","Status":"remove","Time":"2026-11-27T09:19:44.593842191+01:00","Type":"container","Attributes":{"x":"1"}}`: nil,
		`Error: unable to connect`: nil,
	} {
		image, ok := parsePodmanImageEvent(line)
		if want == nil {
			assert.False(t, ok, line)
			continue
		}
		require.True(t, ok, line)
		assert.Equal(t, *want, image, line)
	}
	_, ok := parsePodmanEvent(`{"ID":"` + id + `","Name":"docker.io/library/hello-world:linux","Status":"remove","Type":"image","Attributes":null}`)
	assert.False(t, ok, "the remove of an image is not the one of a container")
}

// The lines docker 29.1.3 prints, captured on a dev cluster node for the
// rmi of a tag, the rmi of an image and a prune: docker names the image
// untagged or deleted by its id alone, and the prune ending a prune names
// none.
func TestParseDockerImageEvent(t *testing.T) {
	const id = "sha256:e2ac70e7319a02c5a477f5825259bd118b94e8b02c279c67afa63adab6d8685b"
	for line, want := range map[string]*imageRemoval{
		`{"Type":"image","Action":"pull","Actor":{"ID":"hello-world:linux","Attributes":{"name":"hello-world"}},"scope":"local","time":1795767566,"timeNano":1795767566148049881}`:               nil,
		`{"Type":"image","Action":"tag","Actor":{"ID":"` + id + `","Attributes":{"name":"prefetch-probe:t1"}},"scope":"local","time":1795767566,"timeNano":1795767566169657242}`:                 nil,
		`{"Type":"image","Action":"untag","Actor":{"ID":"` + id + `","Attributes":{"name":"` + id + `"}},"scope":"local","time":1795767566,"timeNano":1795767566186500480}`:                      {id: id},
		`{"Type":"image","Action":"delete","Actor":{"ID":"` + id + `","Attributes":{"name":"` + id + `"}},"scope":"local","time":1795767566,"timeNano":1795767566208524747}`:                     {id: id},
		`{"Type":"image","Action":"untag","Actor":{"ID":"` + id + `","Attributes":{"name":"` + id + `","prefetch-probe":"1"}},"scope":"local","time":1795767566,"timeNano":1795767566274008837}`: {id: id},
		`{"Type":"image","Action":"untag","Actor":{"ID":"` + id + `","Attributes":{"name":"registry.example.com/app:1"}},"scope":"local"}`:                                                       {name: "registry.example.com/app:1", id: id},
		`{"Type":"image","Action":"prune","Actor":{"ID":"","Attributes":{"reclaimed":"6"}},"scope":"local","time":1795767566,"timeNano":1795767566279771647}`:                                    nil,
		`{"Type":"container","Action":"destroy","Actor":{"ID":"56bccc0e5817","Attributes":{"image":"nginx:1.30-alpine","name":"omtest-evt-d"}},"scope":"local"}`:                                 nil,
	} {
		image, ok := parseDockerImageEvent(line)
		if want == nil {
			assert.False(t, ok, line)
			continue
		}
		require.True(t, ok, line)
		assert.Equal(t, *want, image, line)
	}
}

// pubSpy records the messages published.
type pubSpy struct {
	mu   sync.Mutex
	msgs []pubsub.Messager
}

func (t *pubSpy) Pub(m pubsub.Messager, _ ...pubsub.Label) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.msgs = append(t.msgs, m)
}

func (t *pubSpy) imagesRemoved() []msgbus.ContainerImageRemoved {
	t.mu.Lock()
	defer t.mu.Unlock()
	l := make([]msgbus.ContainerImageRemoved, 0)
	for _, m := range t.msgs {
		if removed, ok := m.(*msgbus.ContainerImageRemoved); ok {
			l = append(l, msgbus.ContainerImageRemoved{Node: removed.Node, Type: removed.Type, Store: removed.Store, Name: removed.Name, ID: removed.ID})
		}
	}
	return l
}

// A stream that starts, or starts again, may have missed removals: it says
// so with a removal naming no image, for the images the node is meant to
// hold to be checked.
func TestAStreamReadyPublishesARemovalNamingNoImage(t *testing.T) {
	m := NewManager(0, nil)
	m.log = plog.NewDefaultLogger()
	spy := &pubSpy{}
	m.publisher = spy
	e := &engine{driverType: "container.docker", counter: evwatch.NewCounter()}
	e.debouncer = evwatch.NewDebouncer(time.Hour, func(string) {})
	defer e.debouncer.Stop()
	m.onStreamReady(e)
	assert.Equal(t, []msgbus.ContainerImageRemoved{{Node: m.localhost, Type: "container.docker"}}, spy.imagesRemoved())
}

// systemPath is the PATH the tests start with, which the scripts of a test
// run their commands from, whatever an earlier test case set.
var systemPath = os.Getenv("PATH")

func installEngine(t *testing.T, name, body string) {
	t.Helper()
	d := t.TempDir()
	if body == "" {
		t.Setenv("PATH", d)
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(d, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	t.Setenv("PATH", d+":"+systemPath)
}

// An engine not installed, or printing no event stream, can not run.
func TestSourceCheck(t *testing.T) {
	m := NewManager(0, nil)
	src := m.sources()["container.docker"]
	installEngine(t, "docker", "")
	require.ErrorIs(t, src.Check(context.Background()), evwatch.ErrUnsupported)

	installEngine(t, "docker", `echo "unknown command: docker events" >&2; exit 1`)
	require.ErrorIs(t, src.Check(context.Background()), evwatch.ErrUnsupported)

	installEngine(t, "docker", `echo "      --format string   Format output using a custom template"`)
	require.NoError(t, src.Check(context.Background()))
}

// The stream of an engine is ready once it ran a while, and emits the
// containers its events say changed state, and the images they say were
// removed.
func TestSourceRun(t *testing.T) {
	var mu sync.Mutex
	installEngine(t, "podman", `[ "$2" = "--help" ] && { echo "--format"; exit 0; }
[ "$*" = "events --format json --filter type=container --filter type=image" ] || exit 2
echo '{"Name":"c1","Status":"create","Type":"container"}'
echo '{"Name":"c1","Status":"start","Type":"container"}'
echo '{"ID":"e2ac","Name":"docker.io/library/nginx:latest","Status":"pull","Type":"image"}'
echo '{"ID":"e2ac","Name":"docker.io/library/nginx:latest","Status":"remove","Type":"image"}'
sleep 60`)
	m := NewManager(0, nil)
	m.log = plog.NewDefaultLogger()
	src := m.sources()["container.podman"]
	var images []imageRemoval
	src.onImage = func(image imageRemoval) {
		mu.Lock()
		images = append(images, image)
		mu.Unlock()
	}
	require.NoError(t, src.Check(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	var readies int
	var names []string
	done := make(chan error, 1)
	go func() {
		done <- src.Run(ctx, func() {
			mu.Lock()
			readies++
			mu.Unlock()
		}, func(name string) {
			mu.Lock()
			names = append(names, name)
			mu.Unlock()
		})
	}()
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return readies == 1 && len(names) == 1 && len(images) == 1
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not stop with its context")
	}
	assert.Equal(t, []string{"c1"}, names, "the create changes nothing")
	assert.Equal(t, []imageRemoval{{name: "docker.io/library/nginx:latest", id: "e2ac"}}, images, "the pull removes nothing")
}
