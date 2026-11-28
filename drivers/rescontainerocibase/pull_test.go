package rescontainerocibase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

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
// failing to, loading the archives it is given, and counting the pulls. An
// archive is the id of the image it brings, and one saying "other" brings
// another image than the one of the resource.
type prefetchExecuter struct {
	Executer

	// leftover is the stopped container the engine holds, if any.
	leftover *InspectData

	hasImage bool
	hasErr   error
	pullErr  error
	loadErr  error
	pulls    int
	loads    []string
	id       string

	removeErr error
	removes   int
}

func (e *prefetchExecuter) HasImage(context.Context) (bool, string, error) {
	if e.hasErr != nil {
		return false, "", e.hasErr
	}
	if e.hasImage {
		if e.id != "" {
			return true, e.id, nil
		}
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

func (e *prefetchExecuter) LoadImage(_ context.Context, r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	e.loads = append(e.loads, string(b))
	if e.loadErr != nil {
		return e.loadErr
	}
	if string(b) != "other" {
		e.hasImage = true
		e.id = string(b)
	}
	return nil
}

func (e *prefetchExecuter) RemoveImage(context.Context) error {
	e.removes++
	if e.removeErr != nil {
		return e.removeErr
	}
	e.hasImage = false
	e.id = ""
	return nil
}

// peer is a node asked for an image: it holds the images of holds, by
// engine and store, as the archive it sends of each, and records what it
// was asked.
type peer struct {
	node     string
	platform string
	holds    map[resource.Pull]string
	id       string
	idKind   string
	err      error
	asked    *[]string
	closed   int
}

func (t *peer) Node() string { return t.node }

func (t *peer) Save(_ context.Context, pull resource.Pull) (*resource.ImageArchive, error) {
	if t.asked != nil {
		*t.asked = append(*t.asked, t.node+" "+pull.String())
	}
	if t.err != nil {
		return nil, t.err
	}
	archive, ok := t.holds[pull]
	if !ok {
		return nil, resource.ErrImageNotFound
	}
	platform := t.platform
	if platform == "" {
		platform = resource.NodePlatform()
	}
	id := t.id
	if id == "" {
		id = archive
	}
	idKind := t.idKind
	if idKind == "" {
		idKind = resource.ImageIDConfig
	}
	return &resource.ImageArchive{ID: id, IDKind: strings.TrimPrefix(idKind, "none"), Platform: platform, Body: &closeCounter{Reader: strings.NewReader(archive), n: &t.closed}}, nil
}

type closeCounter struct {
	io.Reader
	n *int
}

func (t *closeCounter) Close() error {
	*t.n++
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
	// The store of the test names its images by the digest of their
	// configuration, and no engine of the node is asked.
	withIDKind(t, resource.ImageIDConfig)
	return bt
}

// withIDKind sets what the ids of the store of the test are the digest of,
// empty for an engine that does not tell.
func withIDKind(t *testing.T, kind string) {
	t.Helper()
	former := imageIDKind
	imageIDKind = func(context.Context, resource.Pull) string { return kind }
	t.Cleanup(func() { imageIDKind = former })
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
	report, err := bt.Prefetch(context.Background(), nil)
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
	report, err := bt.Prefetch(context.Background(), nil)
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
	report, err := bt.Prefetch(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchFailed, report.State)
	assert.Equal(t, "can't pull image nginx: manifest unknown", report.Error)
	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchFailed, outcome.State)
	assert.Equal(t, report.Error, outcome.Error)

	// The next prefetch replaces the outcome.
	e.pullErr = nil
	_, err = bt.Prefetch(context.Background(), nil)
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
	_, err := bt.Prefetch(context.Background(), nil)
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

// A prefetch that asked nodes for a copy before its registry warns of what
// both said.
func TestStatusWarnsOfWhatThePeersAndTheRegistrySaid(t *testing.T) {
	e := &prefetchExecuter{pullErr: errors.New("manifest unknown")}
	bt := btToPrefetch(t, e)
	bt.Detach = true
	_, err := bt.Prefetch(context.Background(), []resource.ImageSource{&peer{node: "n2"}, &peer{node: "n3", err: errors.New("no route to host")}})
	require.NoError(t, err)
	require.Equal(t, status.Down, bt.Status(context.Background()))
	require.Equal(t, 1, bt.StatusLog().Len())
	assert.Equal(t, "warn: image nginx is not on this node and its last prefetch failed (peers: n2: image not found, n3: no route to host; registry: can't pull image nginx: manifest unknown): a failover here pulls it first, slow or failing if the registry is unreachable", bt.StatusLog().Entries()[0].String())
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

// nginxOn is the key of the image of the test resource in the store of the
// system of an engine.
func nginxOn(engine string) resource.Pull {
	return resource.Pull{Engine: engine, Image: "docker.io/library/nginx:latest"}
}

// A missing image is copied from the first of the nodes holding it, asked
// in the order given, and the registry is not asked: the outcome tells the
// node, and the id of the image loaded, which is the one the node
// announced.
func TestPrefetchCopiesTheImageFromTheFirstPeerHoldingIt(t *testing.T) {
	var asked []string
	e := &prefetchExecuter{}
	bt := btToPrefetch(t, e)
	n2 := &peer{node: "n2", asked: &asked}
	n3 := &peer{node: "n3", asked: &asked, holds: map[resource.Pull]string{nginxOn("docker"): "sha256:fromn3"}}
	n4 := &peer{node: "n4", asked: &asked, holds: map[resource.Pull]string{nginxOn("docker"): "sha256:fromn4"}}

	report, err := bt.Prefetch(context.Background(), []resource.ImageSource{n2, n3, n4})
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchPulled, report.State)
	assert.Equal(t, "peer n3", report.Source)
	assert.Equal(t, "sha256:fromn3", report.ImageID)
	assert.Empty(t, report.Error)
	assert.Zero(t, e.pulls, "the registry is not asked")
	assert.Equal(t, []string{"sha256:fromn3"}, e.loads)
	assert.Equal(t, []string{"n2 docker image docker.io/library/nginx:latest", "n3 docker image docker.io/library/nginx:latest"}, asked)
	assert.Equal(t, 1, n3.closed, "the archive is closed")

	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchPulled, outcome.State)
	assert.Equal(t, "peer n3", outcome.Source, "the outcome recorded tells the source")
}

// With no node holding the image, it is pulled from its registry as with
// no node to ask, and the outcome says so.
func TestPrefetchPullsFromTheRegistryWhenNoPeerHoldsTheImage(t *testing.T) {
	e := &prefetchExecuter{}
	bt := btToPrefetch(t, e)
	report, err := bt.Prefetch(context.Background(), []resource.ImageSource{&peer{node: "n2"}})
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchPulled, report.State)
	assert.Equal(t, resource.PrefetchSourceRegistry, report.Source)
	assert.Equal(t, 1, e.pulls)
	assert.Empty(t, e.loads)

	e = &prefetchExecuter{}
	bt = btToPrefetch(t, e)
	report, err = bt.Prefetch(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchSourceRegistry, report.Source)
	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, resource.PrefetchSourceRegistry, outcome.Source)
}

// A prefetch failed by the nodes and by the registry records what both
// said, each node by its name, and the registry alone when no node was
// asked.
func TestPrefetchRecordsWhatThePeersAndTheRegistrySaid(t *testing.T) {
	e := &prefetchExecuter{pullErr: errors.New("manifest unknown")}
	bt := btToPrefetch(t, e)
	report, err := bt.Prefetch(context.Background(), []resource.ImageSource{
		&peer{node: "n2"},
		&peer{node: "n3", err: errors.New("no route to host")},
	})
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchFailed, report.State)
	assert.Empty(t, report.Source)
	assert.Equal(t, "peers: n2: image not found, n3: no route to host; registry: can't pull image nginx: manifest unknown", report.Error)

	report, err = bt.Prefetch(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "can't pull image nginx: manifest unknown", report.Error)
}

// An image built for another platform is of no use to the node: the archive
// of a node of another one is not loaded, and the node not asked again for
// the store of another engine.
func TestPrefetchDoesNotLoadTheImageOfANodeOfAnotherPlatform(t *testing.T) {
	defer func(former func() string) { nodePlatform = former }(nodePlatform)
	nodePlatform = func() string { return "linux/amd64" }
	resource.RegisterImageStore("podman", CLIImageStore{Bin: "podman"})
	resource.RegisterImageStore("docker", CLIImageStore{Bin: "docker"})

	var asked []string
	e := &prefetchExecuter{pullErr: errors.New("manifest unknown")}
	bt := btToPrefetch(t, e)
	arm := &peer{node: "n2", platform: "linux/arm64", asked: &asked, holds: map[resource.Pull]string{
		nginxOn("docker"): "sha256:arm",
		nginxOn("podman"): "sha256:arm",
	}}
	report, err := bt.Prefetch(context.Background(), []resource.ImageSource{arm})
	require.NoError(t, err)
	assert.Empty(t, e.loads, "nothing of the node is loaded")
	assert.Equal(t, 1, arm.closed, "the archive is left unread, and closed")
	assert.Equal(t, []string{"n2 docker image docker.io/library/nginx:latest"}, asked)
	assert.Equal(t, resource.PrefetchFailed, report.State)
	assert.Equal(t, "peers: n2: a linux/arm64 node, this one is linux/amd64; registry: can't pull image nginx: manifest unknown", report.Error)

	// A node of the platform after it serves the image.
	amd := &peer{node: "n3", platform: "linux/amd64", holds: map[resource.Pull]string{nginxOn("docker"): "sha256:amd"}}
	report, err = bt.Prefetch(context.Background(), []resource.ImageSource{arm, amd})
	require.NoError(t, err)
	assert.Equal(t, "peer n3", report.Source)
	assert.Equal(t, []string{"sha256:amd"}, e.loads)
}

// The image loaded has to be the one the node announced, under the name the
// resource runs: an archive bringing another image, or the image under no
// such name, is a copy that failed, and the next source is asked.
func TestPrefetchRefusesACopyThatIsNotTheImageAnnounced(t *testing.T) {
	e := &prefetchExecuter{pullErr: errors.New("manifest unknown")}
	bt := btToPrefetch(t, e)
	report, err := bt.Prefetch(context.Background(), []resource.ImageSource{
		&peer{node: "n2", id: "sha256:announced", holds: map[resource.Pull]string{nginxOn("docker"): "sha256:loaded"}},
	})
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchFailed, report.State)
	assert.Contains(t, report.Error, "peers: n2: the image loaded is sha256:loaded, not the sha256:announced announced: removed; registry: ",
		"the outcome says what became of the image")
	assert.Equal(t, 1, e.removes, "the name the load gave another image is removed")
	assert.False(t, e.hasImage, "no start runs it")
	outcome, ok := resource.ReadPrefetchOutcome(bt.pullKey())
	require.True(t, ok)
	assert.Equal(t, report.Error, outcome.Error)

	// An image that can not be removed is said to be left there.
	e = &prefetchExecuter{pullErr: errors.New("manifest unknown"), removeErr: errors.New("conflict: image is being used")}
	bt = btToPrefetch(t, e)
	report, err = bt.Prefetch(context.Background(), []resource.ImageSource{
		&peer{node: "n2", id: "sha256:announced", holds: map[resource.Pull]string{nginxOn("docker"): "sha256:loaded"}},
	})
	require.NoError(t, err)
	assert.Contains(t, report.Error, "peers: n2: the image loaded is sha256:loaded, not the sha256:announced announced, and its removal failed: conflict: image is being used; registry: ")

	e = &prefetchExecuter{}
	bt = btToPrefetch(t, e)
	report, err = bt.Prefetch(context.Background(), []resource.ImageSource{
		&peer{node: "n2", holds: map[resource.Pull]string{nginxOn("docker"): "other"}},
		&peer{node: "n3", err: errors.New("no route to host")},
	})
	require.NoError(t, err)
	assert.Equal(t, resource.PrefetchSourceRegistry, report.Source, "the registry is asked after the nodes")
	assert.Equal(t, []string{"other"}, e.loads)

	e = &prefetchExecuter{loadErr: errors.New("no space left on device"), pullErr: errors.New("manifest unknown")}
	bt = btToPrefetch(t, e)
	report, err = bt.Prefetch(context.Background(), []resource.ImageSource{
		&peer{node: "n2", holds: map[resource.Pull]string{nginxOn("docker"): "sha256:abc"}},
	})
	require.NoError(t, err)
	assert.Contains(t, report.Error, "peers: n2: load: no space left on device; registry: ", "a full disk is the error of the load")
}

// Two stores naming an image by the digest of its configuration, or both by
// the one of its manifest, give a copy the id it had: the ids are compared.
// Stores of two kinds, as a docker keeping its images in containerd and one
// that does not, name the image copied differently, and a store may not
// tell its kind: the image under the name the resource runs is then the
// copy, whatever its id, and nothing is removed.
func TestPrefetchComparesTheIDsOfStoresNamingAnImageTheSame(t *testing.T) {
	for name, tc := range map[string]struct {
		local    string
		remote   string
		compared bool
	}{
		"both by the configuration":         {local: resource.ImageIDConfig, remote: resource.ImageIDConfig, compared: true},
		"both by the manifest":              {local: resource.ImageIDManifest, remote: resource.ImageIDManifest, compared: true},
		"this one by the manifest":          {local: resource.ImageIDManifest, remote: resource.ImageIDConfig},
		"the node by the manifest":          {local: resource.ImageIDConfig, remote: resource.ImageIDManifest},
		"the node not telling":              {local: resource.ImageIDConfig, remote: "none"},
		"this one not telling":              {local: "", remote: resource.ImageIDConfig},
		"neither telling":                   {local: "", remote: "none"},
		"this one not telling the manifest": {local: "", remote: resource.ImageIDManifest},
	} {
		t.Run(name, func(t *testing.T) {
			// The image loaded has another id than the one announced.
			e := &prefetchExecuter{pullErr: errors.New("manifest unknown")}
			bt := btToPrefetch(t, e)
			withIDKind(t, tc.local)
			report, err := bt.Prefetch(context.Background(), []resource.ImageSource{
				&peer{node: "n2", id: "sha256:announced", idKind: tc.remote, holds: map[resource.Pull]string{nginxOn("docker"): "sha256:loaded"}},
			})
			require.NoError(t, err)
			if tc.compared {
				assert.Equal(t, resource.PrefetchFailed, report.State)
				assert.Equal(t, 1, e.removes)
				return
			}
			assert.Equal(t, resource.PrefetchPulled, report.State)
			assert.Equal(t, "peer n2", report.Source)
			assert.Equal(t, "sha256:loaded", report.ImageID, "the id is the one of this store")
			assert.Zero(t, e.removes)
			assert.Zero(t, e.pulls)

			// An archive not bringing the image under its name is no
			// copy, compared ids or not.
			e = &prefetchExecuter{pullErr: errors.New("manifest unknown")}
			bt = btToPrefetch(t, e)
			withIDKind(t, tc.local)
			report, err = bt.Prefetch(context.Background(), []resource.ImageSource{
				&peer{node: "n2", idKind: tc.remote, holds: map[resource.Pull]string{nginxOn("docker"): "other"}},
			})
			require.NoError(t, err)
			assert.Equal(t, resource.PrefetchFailed, report.State)
			assert.Contains(t, report.Error, "the archive loaded did not bring image nginx")
		})
	}
}

// The ids of an image as docker and podman print them are the same image.
func TestSameImageID(t *testing.T) {
	assert.True(t, sameImageID("sha256:abc", "abc"))
	assert.True(t, sameImageID("abc", "abc"))
	assert.False(t, sameImageID("sha256:abc", "sha256:abd"))
	assert.False(t, sameImageID("", ""), "no id is no proof")
}

// The nodes are asked for the image in the store of the engine of the
// resource first, all of them, then in the store of the system of the other
// engines, whose archive this one loads: a node holding the image as a
// start here would is preferred to a nearer one holding it another way.
// The store of a user is asked of its engine alone, and so is an image
// naming no registry, which the engines do not read the same.
func TestPrefetchPrefersAPeerOfTheSameEngine(t *testing.T) {
	resource.RegisterImageStore("podman", CLIImageStore{Bin: "podman"})
	resource.RegisterImageStore("docker", CLIImageStore{Bin: "docker"})

	var asked []string
	e := &prefetchExecuter{}
	bt := btToPrefetch(t, e)
	n2 := &peer{node: "n2", asked: &asked, holds: map[resource.Pull]string{nginxOn("podman"): "frompodman"}}
	n3 := &peer{node: "n3", asked: &asked, holds: map[resource.Pull]string{nginxOn("docker"): "sha256:fromdocker"}}
	report, err := bt.Prefetch(context.Background(), []resource.ImageSource{n2, n3})
	require.NoError(t, err)
	assert.Equal(t, "peer n3", report.Source)

	asked = nil
	e = &prefetchExecuter{}
	bt = btToPrefetch(t, e)
	report, err = bt.Prefetch(context.Background(), []resource.ImageSource{n2, &peer{node: "n3", asked: &asked}})
	require.NoError(t, err)
	assert.Equal(t, "peer n2", report.Source, "the archive of another engine is loaded when no node has the one of this engine")
	assert.Equal(t, "frompodman", report.ImageID)
	assert.Equal(t, []string{
		"n2 docker image docker.io/library/nginx:latest",
		"n3 docker image docker.io/library/nginx:latest",
		"n2 podman image docker.io/library/nginx:latest",
	}, asked)

	asked = nil
	e = &prefetchExecuter{}
	bt = btToPrefetch(t, e)
	bt.SetImageStore("podman", "app")
	bt.Image = "ghcr.io/opensvc/pause:1"
	_, err = bt.Prefetch(context.Background(), []resource.ImageSource{&peer{node: "n2", asked: &asked}})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"n2 podman image ghcr.io/opensvc/pause:1 in the store of app",
		"n2 docker image ghcr.io/opensvc/pause:1",
	}, asked)

	// A podman image naming no registry is the one its registries.conf
	// resolves, where docker reads the same name as one of its hub: the
	// docker of a node is not asked for it, and the image it holds under
	// that name is not copied. The registry is asked, as a start would.
	asked = nil
	e = &prefetchExecuter{}
	bt = btToPrefetch(t, e)
	bt.SetImageStore("podman", "")
	n2 = &peer{node: "n2", asked: &asked, holds: map[resource.Pull]string{{Engine: "docker", Image: "nginx:latest"}: "sha256:fromthehub"}}
	report, err = bt.Prefetch(context.Background(), []resource.ImageSource{n2})
	require.NoError(t, err)
	assert.Equal(t, []string{"n2 podman image nginx:latest"}, asked)
	assert.Equal(t, 1, e.pulls)
	assert.Empty(t, e.loads)
	assert.Equal(t, resource.PrefetchPulled, report.State)
	assert.Equal(t, resource.PrefetchSourceRegistry, report.Source)
}

// A copy is bound by the pull_timeout, as a pull is, and no credentials go
// to the nodes: the key they are asked names none.
func TestPrefetchAsksThePeersWithNoCredentialsWithinThePullTimeout(t *testing.T) {
	var asked []resource.Pull
	var deadline bool
	e := &prefetchExecuter{}
	bt := btToPrefetch(t, e)
	bt.RegistryCreds = "creds"
	timeout := time.Minute
	bt.PullTimeout = &timeout
	_, err := bt.Prefetch(context.Background(), []resource.ImageSource{sourceFunc(func(ctx context.Context, pull resource.Pull) (*resource.ImageArchive, error) {
		asked = append(asked, pull)
		_, deadline = ctx.Deadline()
		return nil, resource.ErrImageNotFound
	})})
	require.NoError(t, err)
	require.NotEmpty(t, asked)
	assert.Equal(t, nginxOn("docker"), asked[0])
	for _, pull := range asked {
		assert.Empty(t, pull.Creds)
	}
	assert.True(t, deadline)
	assert.Equal(t, "ns1/sec/creds", bt.pullKey().Creds, "the key of the outcome keeps them")
}

// sourceFunc is a node named n2 answering with a function.
type sourceFunc func(context.Context, resource.Pull) (*resource.ImageArchive, error)

func (t sourceFunc) Node() string { return "n2" }

func (t sourceFunc) Save(ctx context.Context, pull resource.Pull) (*resource.ImageArchive, error) {
	return t(ctx, pull)
}
