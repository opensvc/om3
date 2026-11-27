package rescontainerocibase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/args"
)

// pullSource is the configuration of a resource, as evaluated for a node:
// the options it holds evaluate, the others do not.
type pullSource struct {
	path    naming.Path
	options map[string]string
}

func (t pullSource) Get(option string) (string, bool) {
	s, ok := t.options[option]
	return s, ok
}

func (t pullSource) Path() naming.Path { return t.path }

func (t pullSource) RID() string { return "container#1" }

// prefetchExecuter is an engine holding an image or not, pulling it or
// failing to, and counting the pulls.
type prefetchExecuter struct {
	Executer

	// leftover is the stopped container the engine holds, if any.
	leftover *InspectData

	hasImage bool
	hasErr   error
	pullErr  error
	pulls    int
}

func (e *prefetchExecuter) HasImage(context.Context) (bool, string, error) {
	if e.hasErr != nil {
		return false, "", e.hasErr
	}
	if e.hasImage {
		return true, "sha256:abc", nil
	}
	return false, "", nil
}

func (e *prefetchExecuter) Pull(context.Context) error {
	e.pulls++
	if e.pullErr != nil {
		return e.pullErr
	}
	e.hasImage = true
	return nil
}

// withVarDir points the node at a var directory of the test.
func withVarDir(t *testing.T) {
	t.Helper()
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": t.TempDir()})
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
}

func btToPrefetch(t *testing.T, e *prefetchExecuter) *BT {
	t.Helper()
	withVarDir(t)
	bt := &BT{
		T:     resource.T{ResourceID: &resourceid.T{Name: "container#1"}},
		Path:  naming.Path{Namespace: "ns1", Name: "foo", Kind: naming.KindSvc},
		Image: "nginx",
	}
	bt.SetImageStore("docker", "")
	require.NoError(t, bt.Configure())
	bt.executer = e
	return bt
}

// The Pull of a resource is read from its configuration: the image as its
// engine reads it, the store the driver names, and the secret of the
// namespace of the object as the identity of its credentials.
func TestPullOf(t *testing.T) {
	path := naming.Path{Namespace: "ns1", Name: "foo", Kind: naming.KindSvc}
	for name, tc := range map[string]struct {
		engine   string
		store    string
		options  map[string]string
		expected resource.Pull
		none     bool
	}{
		"an image of the hub": {
			engine:   "docker",
			options:  map[string]string{"image": "nginx", "registry_creds": ""},
			expected: resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"},
		},
		"a short name of podman": {
			engine:   "podman",
			options:  map[string]string{"image": "nginx"},
			expected: resource.Pull{Engine: "podman", Image: "nginx:latest"},
		},
		"a qualified name of podman": {
			engine:   "podman",
			options:  map[string]string{"image": "docker.io/nginx"},
			expected: resource.Pull{Engine: "podman", Image: "docker.io/library/nginx:latest"},
		},
		"the store of a rootless user": {
			engine:   "podman",
			store:    "app",
			options:  map[string]string{"image": "ghcr.io/opensvc/pause:1"},
			expected: resource.Pull{Engine: "podman", Store: "app", Image: "ghcr.io/opensvc/pause:1"},
		},
		"the credentials of a secret": {
			engine:   "docker",
			options:  map[string]string{"image": "registry.example.com/app:1", "registry_creds": "creds"},
			expected: resource.Pull{Engine: "docker", Image: "registry.example.com/app:1", Creds: "ns1/sec/creds"},
		},
		"an image that does not evaluate": {
			engine:  "docker",
			options: map[string]string{},
			none:    true,
		},
		"an empty image": {
			engine:  "docker",
			options: map[string]string{"image": ""},
			none:    true,
		},
		"an image read as an option": {
			engine:  "docker",
			options: map[string]string{"image": "--privileged"},
			none:    true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			pull, ok := PullOf(tc.engine, tc.store, pullSource{path: path, options: tc.options})
			assert.Equal(t, !tc.none, ok)
			assert.Equal(t, tc.expected, pull)
		})
	}
}

// The status reads the outcome of the key the daemon prefetched: the driver
// configured says the key its configuration says.
func TestTheConfiguredDriverSaysTheKeyOfItsConfiguration(t *testing.T) {
	bt := btToPrefetch(t, &prefetchExecuter{})
	bt.RegistryCreds = "creds"
	fromConfig, ok := PullOf("docker", "", pullSource{path: bt.Path, options: map[string]string{"image": "nginx", "registry_creds": "creds"}})
	require.True(t, ok)
	assert.Equal(t, fromConfig, bt.pullKey())
}

// An image the store holds is not pulled again, whatever the pull policy of
// the starts.
func TestPrefetchDoesNotPullAPresentImage(t *testing.T) {
	e := &prefetchExecuter{hasImage: true}
	bt := btToPrefetch(t, e)
	bt.ImagePullPolicy = imagePullPolicyAlways
	report, err := bt.Prefetch(context.Background())
	require.NoError(t, err)
	assert.Zero(t, e.pulls)
	assert.Equal(t, "container#1", report.RID)
	assert.Equal(t, resource.PrefetchPresent, report.State)
	assert.Equal(t, "sha256:abc", report.ImageID)
	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchPresent, outcome.State)
	assert.Equal(t, resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}, outcome.Key)
}

func TestPrefetchPullsAMissingImage(t *testing.T) {
	e := &prefetchExecuter{}
	bt := btToPrefetch(t, e)
	report, err := bt.Prefetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, e.pulls)
	assert.Equal(t, resource.PrefetchPulled, report.State)
	assert.Equal(t, "sha256:abc", report.ImageID)
	assert.False(t, report.UpdatedAt.IsZero())
	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchPulled, outcome.State)
}

// A pull that failed is an outcome, not an error of the command: it is
// recorded with what the engine said, for the status and for the daemon.
func TestPrefetchRecordsAFailedPull(t *testing.T) {
	e := &prefetchExecuter{pullErr: errors.New("manifest unknown")}
	bt := btToPrefetch(t, e)
	report, err := bt.Prefetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchFailed, report.State)
	assert.Equal(t, "can't pull image nginx: manifest unknown", report.Error)
	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchFailed, outcome.State)
	assert.Equal(t, report.Error, outcome.Error)

	// The next prefetch replaces the outcome.
	e.pullErr = nil
	_, err = bt.Prefetch(context.Background())
	require.NoError(t, err)
	outcome, ok = resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchPulled, outcome.State)
	assert.Empty(t, outcome.Error)
}

// Nothing is known of the image of an engine that does not answer: no
// outcome is recorded, and the former one is kept.
func TestPrefetchRecordsNothingOfAnEngineThatDoesNotAnswer(t *testing.T) {
	e := &prefetchExecuter{hasErr: errors.New("the engine does not answer: Cannot connect")}
	bt := btToPrefetch(t, e)
	_, err := bt.Prefetch(context.Background())
	require.ErrorContains(t, err, "the engine does not answer")
	assert.Zero(t, e.pulls)
	_, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	assert.False(t, ok)
}

// The store of an engine is asked with the image inspect a start asks, and
// answers the same of a missing image and of an engine that does not answer.
func TestCLIImageStore(t *testing.T) {
	pull := resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}
	inspect := `test "$*" = "image inspect --format {{.ID}} docker.io/library/nginx:latest" || { echo "unexpected $*" >&2; exit 2; }; `

	present, id, err := CLIImageStore{Bin: fakeEngineScript(t, inspect+"echo sha256:abc")}.Has(context.Background(), pull)
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "sha256:abc", id)

	present, _, err = CLIImageStore{Bin: fakeEngineScript(t, inspect+"echo 'Error response from daemon: No such image: nginx' >&2; exit 1")}.Has(context.Background(), pull)
	require.NoError(t, err)
	assert.False(t, present)

	_, _, err = CLIImageStore{Bin: fakeEngineScript(t, inspect+"echo 'Cannot connect' >&2; exit 1")}.Has(context.Background(), pull)
	require.EqualError(t, err, "the engine does not answer: Cannot connect")

	// The short name of a podman image is asked as written, for podman
	// to resolve it as a start would: asked as an image of the docker
	// hub, one its registries.conf resolves elsewhere would be missing
	// at every check.
	short, ok := PullOf("podman", "", pullSource{options: map[string]string{"image": "nginx"}})
	require.True(t, ok)
	inspectShort := `test "$*" = "image inspect --format {{.ID}} nginx:latest" || { echo "unexpected $*" >&2; exit 2; }; `
	present, id, err = CLIImageStore{Bin: fakeEngineScript(t, inspectShort+"echo sha256:def")}.Has(context.Background(), short)
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "sha256:def", id)

	// The store of a user is not read by the agent as itself, and an
	// engine with none serves none.
	_, _, err = CLIImageStore{Bin: "podman"}.Has(context.Background(), resource.Pull{Engine: "podman", Store: "app", Image: "x"})
	require.Error(t, err)
	_, _, err = CLIImageStore{Bin: "podman"}.Has(context.Background(), resource.Pull{Engine: "podman", Image: "--help"})
	require.Error(t, err)
}

// pullArgs is the argser of an executor whose pull is a shell script.
type pullArgs struct {
	ExecutorArgser
	script string
}

func (a pullArgs) PullArgs() *args.T {
	return args.New("-c", a.script)
}

// What the engine says of a pull that failed is in the error, which the
// status of the resource shows after a failed prefetch.
func TestAFailedPullSaysWhatTheEngineSaid(t *testing.T) {
	e := NewExecutor("/bin/sh", pullArgs{script: "echo pulling; echo 'Error response from daemon: manifest unknown' >&2; exit 1"}, testLogger{})
	err := e.Pull(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "Error response from daemon: manifest unknown")
	require.NoError(t, NewExecutor("/bin/sh", pullArgs{script: "echo pulled >&2"}, testLogger{}).Pull(context.Background()))
}

// Inspect answers there is no container, or the stopped one left: the
// resource is down, where the warning of a failed prefetch is shown.
func (e *prefetchExecuter) Inspect(context.Context) (Inspecter, error) {
	if e.leftover == nil {
		return nil, nil
	}
	return e.leftover, nil
}

// The status of a resource with no container warns of its image when it is
// missing after a prefetch that failed, and only then: a prefetch pending,
// done, or disabled shows nothing, and so does an engine that does not
// answer, which tells nothing of the image.
//
// A stopped container left where rm is set, as after a crash, is removed by
// the start, which then pulls: its resource is warned of as one with no
// container. A stopped container kept is started from what the engine
// holds, with no pull, and is not.
func TestStatusWarnsOfAMissingImageAfterAFailedPrefetch(t *testing.T) {
	failed := &resource.PrefetchOutcome{State: resource.PrefetchFailed, Error: "manifest unknown"}
	for name, tc := range map[string]struct {
		outcome  *resource.PrefetchOutcome
		hasImage bool
		hasErr   error
		prefetch string
		leftover bool
		remove   bool
		warns    bool
	}{
		"no prefetch yet": {},
		"a prefetch that pulled, and a prune since": {
			outcome: &resource.PrefetchOutcome{State: resource.PrefetchPulled},
		},
		"a prefetch that failed, the image missing": {
			outcome: failed,
			warns:   true,
		},
		"a prefetch that failed, asked by the resource": {
			outcome:  failed,
			prefetch: "true",
			warns:    true,
		},
		"a prefetch that failed, the image pulled by a start since": {
			outcome:  failed,
			hasImage: true,
		},
		"a prefetch that failed, disabled since": {
			outcome:  failed,
			prefetch: "false",
		},
		"a prefetch that failed, the engine not answering": {
			outcome: failed,
			hasErr:  errors.New("the engine does not answer"),
		},
		"a prefetch that failed, a stopped container the start removes": {
			outcome:  failed,
			leftover: true,
			remove:   true,
			warns:    true,
		},
		"a prefetch that failed, a stopped container the start removes, the image pulled since": {
			outcome:  failed,
			leftover: true,
			remove:   true,
			hasImage: true,
		},
		"no prefetch yet, a stopped container the start removes": {
			leftover: true,
			remove:   true,
		},
		"a prefetch that failed, a stopped container kept": {
			outcome:  failed,
			leftover: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := &prefetchExecuter{hasImage: tc.hasImage, hasErr: tc.hasErr}
			if tc.leftover {
				// Made of the image the store holds, or of none: the
				// status has no other image to tell from its own.
				_, id, _ := e.HasImage(context.Background())
				e.leftover = &InspectData{Image: id}
			}
			bt := btToPrefetch(t, e)
			bt.Detach = true
			bt.Remove = tc.remove
			bt.ImagePrefetch = tc.prefetch
			if tc.outcome != nil {
				outcome := *tc.outcome
				outcome.Key = bt.pullKey()
				require.NoError(t, outcome.Write())
			}
			require.Equal(t, status.Down, bt.Status(context.Background()))
			var warnings []string
			for _, e := range bt.StatusLog().Entries() {
				warnings = append(warnings, e.String())
			}
			if !tc.warns {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Equal(t, "warn: image nginx is not on this node and its last prefetch failed (manifest unknown): a failover here pulls it first, slow or failing if the registry is unreachable", warnings[0])
		})
	}
}

// The outcome of another key is not the one of the resource: a failed
// prefetch of the image the configuration named before warns of nothing.
func TestStatusIgnoresTheFailedPrefetchOfAnotherImage(t *testing.T) {
	bt := btToPrefetch(t, &prefetchExecuter{})
	bt.Detach = true
	other := resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:1"}
	require.NoError(t, resource.PrefetchOutcome{Key: other, State: resource.PrefetchFailed}.Write())
	require.Equal(t, status.Down, bt.Status(context.Background()))
	assert.Zero(t, bt.StatusLog().Len())
}
