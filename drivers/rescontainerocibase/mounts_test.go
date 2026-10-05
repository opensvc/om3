package rescontainerocibase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/naming"
)

// stagingTest makes a volume head with an html directory, and a staging root,
// both removed after the test, and returns them.
func stagingTest(t *testing.T) (string, string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("a bind mount needs root")
	}
	base := t.TempDir()
	head := filepath.Join(base, "head")
	require.NoError(t, os.MkdirAll(filepath.Join(head, "html"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(head, "html", "index.html"), []byte("volume"), 0644))
	root := filepath.Join(base, "staging")
	previous := stagingRoot
	stagingRoot = root
	t.Cleanup(func() {
		stagingRoot = previous
		_ = filepath.Walk(root, func(p string, _ os.FileInfo, _ error) error {
			_ = unmountAll(p)
			return nil
		})
	})
	return head, root
}

func isMountPoint(t *testing.T, p string) bool {
	t.Helper()
	b, err := os.ReadFile("/proc/self/mountinfo")
	require.NoError(t, err)
	for _, line := range strings.Split(string(b), "\n") {
		if fields := strings.Fields(line); len(fields) > 4 && fields[4] == p {
			return true
		}
	}
	return false
}

func TestAVolumeSourceIsBindMountedOnItsStagingPath(t *testing.T) {
	head, root := stagingTest(t)
	target := filepath.Join(root, "0")
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), target, ""))
	b, err := os.ReadFile(filepath.Join(target, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "volume", string(b))
}

// The staging mount is the directory the agent opened: a link swapped in its
// place afterwards changes nothing of what the engine mounts.
func TestALinkSwappedInAfterTheStagingChangesNothing(t *testing.T) {
	head, root := stagingTest(t)
	target := filepath.Join(root, "0")
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), target, ""))
	require.NoError(t, os.Rename(filepath.Join(head, "html"), filepath.Join(head, "html.old")))
	require.NoError(t, os.Symlink("/", filepath.Join(head, "html")))

	b, err := os.ReadFile(filepath.Join(target, "index.html"))
	require.NoError(t, err)
	assert.Equal(t, "volume", string(b))
	assert.NoFileExists(t, filepath.Join(target, "etc", "passwd"))
}

// A link planted before the staging is refused, and nothing is mounted.
func TestAVolumeSourceLeadingOutOfTheHeadIsNotStaged(t *testing.T) {
	head, root := stagingTest(t)
	require.NoError(t, os.RemoveAll(filepath.Join(head, "html")))
	require.NoError(t, os.Symlink("/", filepath.Join(head, "html")))
	target := filepath.Join(root, "0")

	assert.Error(t, stageMount(head, filepath.Join(head, "html"), target, ""))
	assert.Error(t, stageMount(head, filepath.Join(head, "html", "etc"), target, ""))
	assert.Error(t, stageMount(head, filepath.Join(head, "..", ".."), target, ""))
	if _, err := os.Stat(target); err == nil {
		assert.False(t, isMountPoint(t, target))
	}
}

func TestAMissingVolumeSourceIsMadeInTheHead(t *testing.T) {
	head, root := stagingTest(t)
	outside := filepath.Join(filepath.Dir(head), "outside")
	require.NoError(t, os.Mkdir(outside, 0755))
	require.NoError(t, os.Symlink(outside, filepath.Join(head, "data")))

	assert.Error(t, stageMount(head, filepath.Join(head, "data", "new"), filepath.Join(root, "0"), ""))
	assert.NoDirExists(t, filepath.Join(outside, "new"), "the missing source is not made through the link")

	require.NoError(t, stageMount(head, filepath.Join(head, "cache"), filepath.Join(root, "1"), ""))
	assert.DirExists(t, filepath.Join(head, "cache"))
	assert.True(t, isMountPoint(t, filepath.Join(root, "1")))
}

func TestAVolumeFileIsStagedOnAFile(t *testing.T) {
	head, root := stagingTest(t)
	target := filepath.Join(root, "0")
	require.NoError(t, stageMount(head, filepath.Join(head, "html", "index.html"), target, ""))
	b, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "volume", string(b))
}

func TestAVolumeWithNoHeadIsNotStaged(t *testing.T) {
	_, root := stagingTest(t)
	assert.Error(t, stageMount("", "/etc", filepath.Join(root, "0"), ""))
}

// A container an agent without staging mounts started has no staging
// directory, and removing its staging mounts is nothing to do.
func TestUnstagingAContainerWithNoStagingMountsIsNothing(t *testing.T) {
	stagingTest(t)
	bt := &BT{}
	bt.Path = naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "web"}
	require.NoError(t, bt.SetRID("container#1"))
	assert.NoError(t, bt.unstageVolumeMounts())
}

func TestUnstagingRemovesTheMountsAndTheirDirectory(t *testing.T) {
	head, _ := stagingTest(t)
	bt := &BT{}
	bt.Path = naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "web"}
	require.NoError(t, bt.SetRID("container#1"))
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), bt.stagingPath(0), ""))
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), bt.stagingPath(0), ""), "a mount stacked by a start that did not unstage")
	require.NoError(t, bt.unstageVolumeMounts())
	assert.NoDirExists(t, bt.stagingDir())
	assert.NoDirExists(t, filepath.Join(stagingRoot, "root"), "the empty directories of the object go too")
	assert.DirExists(t, stagingRoot)
}

func TestAContainerMountingItsVolumeByPathIsNotStaged(t *testing.T) {
	bt := &BT{VolumeMounts: []string{"/srv/data:/data", "web/html:/usr/share/nginx/html:ro"}}
	bt.Path = naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "web"}
	require.NoError(t, bt.SetRID("container#1"))
	byPath := &InspectData{InspectDataMounts: []InspectDataMount{
		{Type: "bind", Source: "/srv/data"},
		{Type: "bind", Source: "/srv/web.root.vol.c/html"},
	}}
	assert.False(t, bt.mountsStaged(byPath))
	staged := &InspectData{InspectDataMounts: []InspectDataMount{
		{Type: "bind", Source: "/srv/data"},
		{Type: "bind", Source: bt.stagingPath(1)},
	}}
	assert.True(t, bt.mountsStaged(staged))
}

func TestTheStagingDirectoriesAreTraversableAndNothingElse(t *testing.T) {
	_, root := stagingTest(t)
	dir := filepath.Join(root, "root", "svc", "web", "container#1")
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, traversable(dir))
	for p := dir; ; p = filepath.Dir(p) {
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0711), info.Mode().Perm(), p)
		if p == root {
			break
		}
	}
	parent, err := os.Stat(filepath.Dir(root))
	require.NoError(t, err)
	assert.NotEqual(t, os.FileMode(0711), parent.Mode().Perm(), "nothing above the staging root is changed")
	assert.Error(t, traversable(filepath.Dir(root)))
}

func TestTheStagingPropagationIsTheOneTheEntryAsks(t *testing.T) {
	for opt, want := range map[string]string{
		"":             "private",
		"rw":           "private",
		"ro,rprivate":  "private",
		"rw,rshared":   "shared",
		"shared":       "shared",
		"rw,rslave":    "slave",
		"ro,slave,z":   "slave",
		"rw,rsharedx":  "private",
		"rw,Z,private": "private",
	} {
		_, name := stagingPropagation(opt)
		assert.Equal(t, want, name, "options %q", opt)
	}
}

// sharedHead makes the volume head a shared mount, as the mount of a volume
// is on a node whose root mount is shared.
func sharedHead(t *testing.T, head string) {
	t.Helper()
	require.NoError(t, unix.Mount(head, head, "", unix.MS_BIND, ""))
	t.Cleanup(func() { _ = unmountAll(head) })
	require.NoError(t, unix.Mount("", head, "", unix.MS_SHARED, ""))
}

// propagationOf returns the optional fields of the mount at p in
// /proc/self/mountinfo, as "shared:12 master:7", empty for a private mount.
func propagationOf(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile("/proc/self/mountinfo")
	require.NoError(t, err)
	found, fields := false, []string{}
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.Fields(line)
		if len(l) < 7 || l[4] != p {
			continue
		}
		// The last mount on p is the one seen through it.
		found, fields = true, []string{}
		for _, f := range l[6:] {
			if f == "-" {
				break
			}
			fields = append(fields, f)
		}
	}
	require.True(t, found, "%s is not a mount point", p)
	return strings.Join(fields, " ")
}

func TestAStagingMountIsPrivateByDefault(t *testing.T) {
	head, root := stagingTest(t)
	sharedHead(t, head)
	target := filepath.Join(root, "0")
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), target, "rw"))
	assert.Equal(t, "", propagationOf(t, target))
}

// A sidecar mounting a FUSE filesystem in a volume asks rshared, and the
// mount it makes reaches the volume, as the staging mount is a peer of it.
func TestAStagingMountAskedSharedIsAPeerOfTheVolume(t *testing.T) {
	head, root := stagingTest(t)
	sharedHead(t, head)
	target := filepath.Join(root, "0")
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), target, "rw,rshared"))
	assert.Equal(t, propagationOf(t, head), propagationOf(t, target))
	require.NoError(t, unix.Mount("none", target, "tmpfs", 0, ""))
	assert.True(t, isMountPoint(t, filepath.Join(head, "html")), "a mount made on the staging mount reaches the volume")
}

// A container using what a sidecar mounts in a volume asks rslave, and sees
// the mounts made in the volume, sending none back.
func TestAStagingMountAskedSlaveReceivesTheMountsOfTheVolume(t *testing.T) {
	head, root := stagingTest(t)
	sharedHead(t, head)
	target := filepath.Join(root, "0")
	require.NoError(t, stageMount(head, filepath.Join(head, "html"), target, "rw,rslave"))
	assert.True(t, strings.HasPrefix(propagationOf(t, target), "master:"), propagationOf(t, target))
	require.NoError(t, unix.Mount("none", filepath.Join(head, "html"), "tmpfs", 0, ""))
	t.Cleanup(func() { _ = unmountAll(filepath.Join(head, "html")) })
	b, err := os.ReadFile("/proc/self/mountinfo")
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(b), " "+target+" "), "the mount made in the volume is stacked on the staging mount")
	require.NoError(t, unmountAll(target))
	assert.True(t, isMountPoint(t, filepath.Join(head, "html")), "a container using what a sidecar mounts leaves the mount to the others when its staging mount goes")
}
