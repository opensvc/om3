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
	"github.com/opensvc/om3/v3/util/plog"
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

func installEngine(t *testing.T, name, body string) {
	t.Helper()
	d := t.TempDir()
	if body == "" {
		t.Setenv("PATH", d)
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(d, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	t.Setenv("PATH", d+":"+os.Getenv("PATH"))
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
// containers its events say changed state.
func TestSourceRun(t *testing.T) {
	installEngine(t, "podman", `[ "$2" = "--help" ] && { echo "--format"; exit 0; }
[ "$1 $2 $3 $4 $5" = "events --format json --filter type=container" ] || exit 2
echo '{"Name":"c1","Status":"create","Type":"container"}'
echo '{"Name":"c1","Status":"start","Type":"container"}'
sleep 60`)
	m := NewManager(0, nil)
	m.log = plog.NewDefaultLogger()
	src := m.sources()["container.podman"]
	require.NoError(t, src.Check(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
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
		return readies == 1 && len(names) == 1
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not stop with its context")
	}
	assert.Equal(t, []string{"c1"}, names, "the create changes nothing")
}
