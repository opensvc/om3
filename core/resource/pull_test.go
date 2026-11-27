package resource

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// withVarDir points the node at a var directory of the test.
func withVarDir(t *testing.T) {
	t.Helper()
	rawconfig.Load(map[string]string{"OSVC_ROOT_PATH": t.TempDir()})
	t.Cleanup(func() { rawconfig.Load(map[string]string{}) })
}

// Two resources naming the same image their own way are one pull: the key
// holds the reference as docker reads it.
func TestNormalizeImage(t *testing.T) {
	for image, expected := range map[string]string{
		"nginx":                              "docker.io/library/nginx:latest",
		"nginx:1.27":                         "docker.io/library/nginx:1.27",
		"library/nginx":                      "docker.io/library/nginx:latest",
		"docker.io/nginx":                    "docker.io/library/nginx:latest",
		"index.docker.io/library/nginx":      "docker.io/library/nginx:latest",
		"opensvc/pause":                      "docker.io/opensvc/pause:latest",
		"ghcr.io/opensvc/pause":              "ghcr.io/opensvc/pause:latest",
		" ghcr.io/opensvc/pause:1 ":          "ghcr.io/opensvc/pause:1",
		"localhost:5000/app/web:v2":          "localhost:5000/app/web:v2",
		"nginx@sha256:" + testDigest:         "docker.io/library/nginx@sha256:" + testDigest,
		"nginx:1.27@sha256:" + testDigest:    "docker.io/library/nginx@sha256:" + testDigest,
		"docker-archive:/srv/images/app.tar": "docker-archive:/srv/images/app.tar",
		"Not A Reference":                    "Not A Reference",
	} {
		assert.Equalf(t, expected, NormalizeImage(image), "image %q", image)
	}
}

// An engine resolving the short names its own way is not told the image is
// one of the docker hub: the name with no registry is kept as written, with
// the tag or the digest the engine reads, and two resources naming the same
// qualified image their own way are still one pull.
func TestNormalizeQualifiedImage(t *testing.T) {
	for image, expected := range map[string]string{
		"nginx":                           "nginx:latest",
		"nginx:1.27":                      "nginx:1.27",
		" nginx ":                         "nginx:latest",
		"library/nginx":                   "library/nginx:latest",
		"opensvc/pause":                   "opensvc/pause:latest",
		"opensvc/pause:1":                 "opensvc/pause:1",
		"nginx@sha256:" + testDigest:      "nginx@sha256:" + testDigest,
		"nginx:1.27@sha256:" + testDigest: "nginx@sha256:" + testDigest,
		"docker.io/nginx":                 "docker.io/library/nginx:latest",
		"index.docker.io/library/nginx":   "docker.io/library/nginx:latest",
		"ghcr.io/opensvc/pause":           "ghcr.io/opensvc/pause:latest",
		" ghcr.io/opensvc/pause:1 ":       "ghcr.io/opensvc/pause:1",
		"localhost/app":                   "localhost/app:latest",
		"localhost:5000/app/web:v2":       "localhost:5000/app/web:v2",
		"ghcr.io/opensvc/pause:1@sha256:" + testDigest: "ghcr.io/opensvc/pause@sha256:" + testDigest,
		"docker-archive:/srv/images/app.tar":           "docker-archive:/srv/images/app.tar",
		"Not A Reference":                              "Not A Reference",
	} {
		assert.Equalf(t, expected, NormalizeQualifiedImage(image), "image %q", image)
	}
}

const testDigest = "5e22040d441e5fb3aed38368acbe8486b575d7018df38dbdfbc7311fbb2ef3a9"

// Every member of the key tells two pulls apart, and none is read as part of
// its neighbour.
func TestPullHash(t *testing.T) {
	pull := Pull{Engine: "podman", Store: "app", Image: "docker.io/library/nginx:latest", Creds: "ns1/sec/creds"}
	assert.Equal(t, pull.Hash(), pull.Hash())
	assert.Len(t, pull.Hash(), 64)
	seen := map[string]Pull{pull.Hash(): pull}
	for _, other := range []Pull{
		{Engine: "docker", Store: "app", Image: "docker.io/library/nginx:latest", Creds: "ns1/sec/creds"},
		{Engine: "podman", Store: "", Image: "docker.io/library/nginx:latest", Creds: "ns1/sec/creds"},
		{Engine: "podman", Store: "app", Image: "docker.io/library/nginx:1", Creds: "ns1/sec/creds"},
		{Engine: "podman", Store: "app", Image: "docker.io/library/nginx:latest", Creds: ""},
		{Engine: "podman", Store: "appdocker.io", Image: "/library/nginx:latest", Creds: "ns1/sec/creds"},
	} {
		if same, ok := seen[other.Hash()]; ok {
			t.Errorf("%+v and %+v have the same hash", same, other)
		}
		seen[other.Hash()] = other
	}
}

// The outcome of a prefetch is recorded by key, replaced by the next one of
// the key, and forgotten with it.
func TestPrefetchOutcomeFile(t *testing.T) {
	withVarDir(t)
	pull := Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}
	other := Pull{Engine: "podman", Image: "docker.io/library/nginx:latest"}

	_, ok := ReadPrefetchOutcome(pull)
	require.False(t, ok, "no outcome is recorded yet")

	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	failed := PrefetchOutcome{Key: pull, State: PrefetchFailed, Error: "registry unreachable", UpdatedAt: at}
	require.NoError(t, failed.Write())
	got, ok := ReadPrefetchOutcome(pull)
	require.True(t, ok)
	assert.Equal(t, failed, got)
	_, ok = ReadPrefetchOutcome(other)
	assert.False(t, ok, "the outcome of a key is not the one of another")

	pulled := PrefetchOutcome{Key: pull, State: PrefetchPulled, ImageID: "sha256:abc", UpdatedAt: at}
	require.NoError(t, pulled.Write())
	got, ok = ReadPrefetchOutcome(pull)
	require.True(t, ok)
	assert.Equal(t, pulled, got)

	// A file that is no outcome is listed with no key, for its removal.
	stray := filepath.Join(PrefetchDir(), "docker", "stray.json")
	require.NoError(t, os.WriteFile(stray, []byte("{"), 0o644))
	// An outcome being written is not one to list, nor to remove.
	require.NoError(t, os.WriteFile(filepath.Join(PrefetchDir(), "docker", ".x.json.123"), []byte("{"), 0o644))
	outcomes := PrefetchOutcomes()
	require.Len(t, outcomes, 2)
	assert.Equal(t, PrefetchOutcome{}, outcomes[stray])
	require.NoError(t, RemovePrefetchOutcomeFile(stray))
	require.Error(t, RemovePrefetchOutcomeFile(filepath.Join(PrefetchDir(), "stray.json")))
	outcomes = PrefetchOutcomes()
	require.Len(t, outcomes, 1)
	for _, outcome := range outcomes {
		assert.Equal(t, pulled, outcome)
	}

	require.NoError(t, RemovePrefetchOutcome(pull))
	require.NoError(t, RemovePrefetchOutcome(pull), "forgetting twice is not an error")
	_, ok = ReadPrefetchOutcome(pull)
	assert.False(t, ok)
	assert.Empty(t, PrefetchOutcomes())
}

// The engine is a word of a configuration: it does not take the outcome out
// of the directory of the outcomes.
func TestPrefetchOutcomeFileStaysInItsDirectory(t *testing.T) {
	withVarDir(t)
	pull := Pull{Engine: "../../../etc", Image: "x"}
	require.NoError(t, PrefetchOutcome{Key: pull, State: PrefetchFailed}.Write())
	outcomes := PrefetchOutcomes()
	require.Len(t, outcomes, 1)
	for path := range outcomes {
		assert.Equal(t, filepath.Join(PrefetchDir(), "etc"), filepath.Dir(path))
	}
}
