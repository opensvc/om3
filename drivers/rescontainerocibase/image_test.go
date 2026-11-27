package rescontainerocibase

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/args"
)

// imageArgs is the argser of an executor whose engine is a shell script: the
// image inspect is the script run with "inspect", and the version asked of
// the engine the script run with "version".
type imageArgs struct {
	ExecutorArgser
}

func (a imageArgs) HasImageArgs() *args.T {
	return args.New("inspect")
}

// fakeEngine writes an engine answering an image inspect with what inspect
// does, and its version with what version does, both shell commands.
func fakeEngine(t *testing.T, inspect, version string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "engine")
	script := "#!/bin/sh\ncase \"$1\" in\ninspect) " + inspect + ";;\nversion) " + version + ";;\nesac\n"
	require.NoError(t, os.WriteFile(p, []byte(script), 0o755))
	return p
}

// fakeEngineScript writes an engine whose version fails, and doing script, a
// shell command, of anything else.
func fakeEngineScript(t *testing.T, script string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "engine")
	script = "#!/bin/sh\nif test \"$1\" = version; then exit 1; fi\n" + script + "\n"
	require.NoError(t, os.WriteFile(p, []byte(script), 0o755))
	return p
}

// A missing image and an engine that does not answer exit the same: the
// first is pulled by a start, the second fails it, and a prefetch retries it
// later rather than recording a missing image.
func TestHasImageTellsAMissingImageFromAnEngineThatDoesNotAnswer(t *testing.T) {
	for name, tc := range map[string]struct {
		inspect string
		version string
		has     bool
		id      string
		err     string
	}{
		"an image the store holds": {
			inspect: "echo sha256:abc",
			version: "exit 0",
			has:     true,
			id:      "sha256:abc",
		},
		"an image docker does not hold": {
			inspect: "echo 'Error response from daemon: No such image: foo:latest' >&2; exit 1",
			version: "exit 1",
		},
		"an image podman does not hold": {
			inspect: "echo 'Error: foo: image not known' >&2; exit 125",
			version: "exit 1",
		},
		"a docker whose daemon is down": {
			inspect: "echo 'WARNING: a warning'>&2; echo 'Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?' >&2; exit 1",
			version: "echo 'Cannot connect to the Docker daemon' >&2; exit 1",
			err:     "the engine does not answer: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?",
		},
		"a message not known, from an engine that answers": {
			inspect: "echo 'Error: reference does not exist' >&2; exit 1",
			version: "exit 0",
		},
		"a failure saying nothing, from an engine that does not answer": {
			inspect: "exit 1",
			version: "echo 'no engine' >&2; exit 1",
			err:     "the engine does not answer: exit status 1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := NewExecutor(fakeEngine(t, tc.inspect, tc.version), imageArgs{}, testLogger{})
			has, id, err := e.HasImage(context.Background())
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.has, has)
			assert.Equal(t, tc.id, id)
		})
	}
}

// An engine that is not installed is reported as it is, without asking the
// version of what can not run.
func TestHasImageReportsAnEngineThatCanNotRun(t *testing.T) {
	e := NewExecutor(filepath.Join(t.TempDir(), "none"), imageArgs{}, testLogger{})
	has, _, err := e.HasImage(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.False(t, has)
}

// A check the context ended says so, and not that the image is missing.
func TestHasImageReportsAnEndedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := NewExecutor(fakeEngine(t, "exit 1", "exit 0"), imageArgs{}, testLogger{})
	_, _, err := e.HasImage(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

// A start reports an engine that does not answer, rather than trying a pull
// bound to fail and reporting that.
func TestAStartReportsAnEngineThatDoesNotAnswer(t *testing.T) {
	bt := btWithTimeouts(t, 0, 0, false)
	bt.executer = NewExecutor(fakeEngine(t, "echo 'Cannot connect' >&2; exit 1", "exit 1"), imageArgs{}, testLogger{})
	err := bt.pullAndRun(context.Background())
	require.EqualError(t, err, "unable to detect if image ghcr.io/opensvc/foo:1 exists locally: the engine does not answer: Cannot connect")
}
