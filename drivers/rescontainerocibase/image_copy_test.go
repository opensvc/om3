package rescontainerocibase

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/args"
)

// copyEngine writes an engine holding the image "docker.io/library/nginx:latest"
// alone, whose save writes an archive naming who ran it, and whose load
// records its arguments, who ran it and what it read in the "loaded" file
// of the returned directory.
func copyEngine(t *testing.T) (bin, dir string) {
	t.Helper()
	dir = t.TempDir()
	// The engine is run, and its record written, by another user in the
	// tests demoting it.
	require.NoError(t, os.Chmod(filepath.Dir(dir), 0o755))
	require.NoError(t, os.Chmod(dir, 0o777))
	bin = filepath.Join(dir, "engine")
	script := `#!/bin/sh
dir=` + dir + `
case "$1 $2" in
"image inspect")
	test "$3 $4" = "--format {{.ID}}" || { echo "unexpected $*" >&2; exit 2; }
	if test "$5" = docker.io/library/nginx:latest; then echo sha256:abc; exit 0; fi
	echo "Error response from daemon: No such image: $5" >&2; exit 1;;
"image save")
	test $# = 3 || { echo "unexpected $*" >&2; exit 2; }
	if test "$3" = docker.io/library/nginx:latest; then printf 'archive of %s by %s' "$3" "$(id -u)"; exit 0; fi
	echo "warning: something" >&2
	echo "Error response from daemon: reference does not exist" >&2; exit 1;;
"image rm")
	test $# = 3 || { echo "unexpected $*" >&2; exit 2; }
	echo "$3 by $(id -u)" >$dir/removed
	if test "$3" = busy; then echo "Error response from daemon: conflict: image is being used" >&2; exit 1; fi;;
"info --format")
	test "$3" = "{{.DriverStatus}}" || { echo "unexpected $*" >&2; exit 2; }
	if test -e $dir/down; then echo "Cannot connect to the Docker daemon" >&2; exit 1; fi
	if test -e $dir/containerd; then echo "[[driver-type io.containerd.snapshotter.v1]]"; else echo "[[Backing Filesystem extfs] [Supports d_type true]]"; fi;;
"image load")
	test $# = 2 || { echo "unexpected $*" >&2; exit 2; }
	rm -f $dir/loaded
	{ echo "by $(id -u) $XDG_RUNTIME_DIR"; cat; } >$dir/loaded
	if grep -q broken $dir/loaded; then echo "Error: payload does not match any of the supported image formats" >&2; exit 125; fi;;
*)
	echo "unexpected $*" >&2; exit 2;;
esac
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return bin, dir
}

// The save of a store writes the archive of the image as the engine makes
// it, and tells the id of the image saved. A store that does not hold the
// image says so, and writes nothing: the requester then asks another node.
func TestCLIImageStoreSave(t *testing.T) {
	bin, _ := copyEngine(t)
	store := CLIImageStore{Bin: bin}
	var b bytes.Buffer

	id, err := store.Save(context.Background(), resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}, &b)
	require.NoError(t, err)
	assert.Equal(t, "sha256:abc", id)
	assert.Equal(t, "archive of docker.io/library/nginx:latest by "+uid(), b.String())

	b.Reset()
	_, err = store.Save(context.Background(), resource.Pull{Engine: "docker", Image: "registry.example.com/app:1"}, &b)
	require.ErrorIs(t, err, resource.ErrImageNotFound)
	assert.Zero(t, b.Len())

	_, err = store.Save(context.Background(), resource.Pull{Engine: "docker", Image: "--output=/etc/passwd"}, &b)
	require.ErrorContains(t, err, "invalid image", "a reference is never read as an option of the engine")
	_, err = store.Save(context.Background(), resource.Pull{Engine: "docker", Store: "app", Image: "docker.io/library/nginx:latest"}, &b)
	require.ErrorContains(t, err, "is not the one of the system", "an engine with no store of a user serves none")
}

// A save that fails says what the engine said last, not its warnings.
func TestAFailedSaveSaysWhatTheEngineSaid(t *testing.T) {
	bin, _ := copyEngine(t)
	cmd, err := CLIImageStore{Bin: bin}.command(context.Background(), resource.Pull{Image: "x"}, imageSaveArgs("x")...)
	require.NoError(t, err)
	err = saveImage(context.Background(), cmd, &bytes.Buffer{})
	require.ErrorContains(t, err, "Error response from daemon: reference does not exist")
	assert.NotContains(t, err.Error(), "warning")
}

// The load of a store hands the archive to the engine, which is the one
// refusing what is no image.
func TestCLIImageStoreLoad(t *testing.T) {
	bin, dir := copyEngine(t)
	store := CLIImageStore{Bin: bin}
	pull := resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}

	require.NoError(t, store.Load(context.Background(), pull, strings.NewReader("the archive")))
	b, err := os.ReadFile(filepath.Join(dir, "loaded"))
	require.NoError(t, err)
	assert.Equal(t, "by "+uid()+" "+os.Getenv("XDG_RUNTIME_DIR")+"\nthe archive", string(b))

	err = store.Load(context.Background(), pull, strings.NewReader("broken"))
	require.ErrorContains(t, err, "payload does not match any of the supported image formats")
}

// The store of a user is saved from and loaded into as that user, who is the
// one podman finds the store of: the agent as itself would read and fill
// the store of the system.
func TestCLIImageStoreReachesTheStoreOfAUserAsThatUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("running a command as another user needs root")
	}
	bin, dir := copyEngine(t)
	var asked []string
	store := CLIImageStore{Bin: bin, Credential: func(store string) (*Credential, error) {
		asked = append(asked, store)
		return nobody, nil
	}}
	pull := resource.Pull{Engine: "podman", Store: "app", Image: "docker.io/library/nginx:latest"}

	present, _, err := store.Has(context.Background(), pull)
	require.NoError(t, err)
	assert.True(t, present)

	var b bytes.Buffer
	_, err = store.Save(context.Background(), pull, &b)
	require.NoError(t, err)
	assert.Equal(t, "archive of docker.io/library/nginx:latest by 65534", b.String())

	require.NoError(t, store.Load(context.Background(), pull, strings.NewReader("the archive")))
	loaded, err := os.ReadFile(filepath.Join(dir, "loaded"))
	require.NoError(t, err)
	assert.Equal(t, "by 65534 /run/user/65534\nthe archive", string(loaded))
	assert.Subset(t, asked, []string{"app"})
	assert.NotContains(t, asked, "")
}

// The load of the executor of a container runs as its other commands do,
// with the arguments the engines share: the archive of a rootless container
// goes to the store of its user, where its start looks for the image.
func TestExecutorLoadImage(t *testing.T) {
	bin, dir := copyEngine(t)
	loaded := func() string {
		b, err := os.ReadFile(filepath.Join(dir, "loaded"))
		require.NoError(t, err)
		return string(b)
	}

	e := NewExecutor(bin, credArgs{}, testLogger{})
	require.NoError(t, e.LoadImage(context.Background(), strings.NewReader("the archive")))
	assert.Equal(t, "by "+uid()+" "+os.Getenv("XDG_RUNTIME_DIR")+"\nthe archive", loaded())

	err := e.LoadImage(context.Background(), strings.NewReader("broken"))
	require.ErrorContains(t, err, "payload does not match any of the supported image formats")

	if os.Geteuid() != 0 {
		t.Skip("running a command as another user needs root")
	}
	e = NewExecutor(bin, credArgs{cred: nobody}, testLogger{})
	require.NoError(t, e.LoadImage(context.Background(), strings.NewReader("the archive")))
	assert.Equal(t, "by 65534 /run/user/65534\nthe archive", loaded())
}

// A load whose context ends reports the context, not the kill of the
// engine.
func TestALoadCutByItsContextReportsTheContext(t *testing.T) {
	bin, _ := copyEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := CLIImageStore{Bin: bin}.Load(ctx, resource.Pull{Image: "x"}, strings.NewReader("the archive"))
	require.ErrorIs(t, err, context.Canceled)
}

// uid is the user the test runs as, as the engine of the test prints it.
func uid() string {
	return strconv.Itoa(os.Geteuid())
}

// removeArgs is the argser of an executor removing the image named.
type removeArgs struct {
	credArgs
	image string
}

func (a removeArgs) RemoveImageArgs() *args.T {
	return args.New(imageRemoveArgs(a.image)...)
}

// The executor of a container removes the name of its image from the store,
// and says what the engine said when it does not.
func TestExecutorRemoveImage(t *testing.T) {
	bin, dir := copyEngine(t)
	e := NewExecutor(bin, removeArgs{image: "registry.example.com/app:1"}, testLogger{})
	require.NoError(t, e.RemoveImage(context.Background()))
	b, err := os.ReadFile(filepath.Join(dir, "removed"))
	require.NoError(t, err)
	assert.Equal(t, "registry.example.com/app:1 by "+uid()+"\n", string(b))

	e = NewExecutor(bin, removeArgs{image: "busy"}, testLogger{})
	require.ErrorContains(t, e.RemoveImage(context.Background()), "conflict: image is being used")
}

// The ids of a store are the digest of the configuration of its images,
// except in a docker keeping its images in containerd, where they are the
// one of their manifest: the daemon is asked. One that does not answer
// tells no kind, and no id of it is then compared.
func TestCLIImageStoreIDKind(t *testing.T) {
	bin, dir := copyEngine(t)
	pull := resource.Pull{Engine: "docker", Image: "docker.io/library/nginx:latest"}

	kind, err := CLIImageStore{Bin: bin}.IDKind(context.Background(), pull)
	require.NoError(t, err)
	assert.Equal(t, resource.ImageIDConfig, kind, "an engine with one kind of id is not asked")

	docker := CLIImageStore{Bin: bin, ManifestIDs: DockerManifestIDs}
	kind, err = docker.IDKind(context.Background(), pull)
	require.NoError(t, err)
	assert.Equal(t, resource.ImageIDConfig, kind)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "containerd"), nil, 0o644))
	kind, err = docker.IDKind(context.Background(), pull)
	require.NoError(t, err)
	assert.Equal(t, resource.ImageIDManifest, kind)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "down"), nil, 0o644))
	kind, err = docker.IDKind(context.Background(), pull)
	require.ErrorContains(t, err, "Cannot connect to the Docker daemon")
	assert.Empty(t, kind)
}
