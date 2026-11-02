package rescontainerkvm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/plog"
)

// allowLocalURLs opens the node policy to the test servers.
func allowLocalURLs(t *testing.T) {
	url, cidr := rawconfig.SSRFAllowedURL, rawconfig.SSRFAllowedCIDR
	rawconfig.SSRFAllowedURL, rawconfig.SSRFAllowedCIDR = []string{"*"}, []string{"127.0.0.0/8"}
	t.Cleanup(func() { rawconfig.SSRFAllowedURL, rawconfig.SSRFAllowedCIDR = url, cidr })
}

func TestDownloadImage(t *testing.T) {
	allowLocalURLs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/image" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("content"))
	}))
	defer srv.Close()
	dir := filepath.Join(t.TempDir(), "var")

	p, err := downloadImage(context.Background(), srv.URL+"/image", dir, 7)
	require.NoError(t, err)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "content", string(b))
	require.NoError(t, os.Remove(p))

	_, err = downloadImage(context.Background(), srv.URL+"/none", dir, 7)
	assert.ErrorContains(t, err, "404")
	_, err = downloadImage(context.Background(), srv.URL+"/image", dir, 6)
	assert.ErrorContains(t, err, "larger than", "what fits no disk does not fill the node")
	l, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, l, "a failed download leaves no file")
}

func TestDownloadImageObeysThePolicy(t *testing.T) {
	url := rawconfig.SSRFAllowedURL
	rawconfig.SSRFAllowedURL = nil
	defer func() { rawconfig.SSRFAllowedURL = url }()
	_, err := downloadImage(context.Background(), "http://127.0.0.1:1/image", t.TempDir(), 1)
	assert.ErrorContains(t, err, "reject url")
}

func TestWriteImage(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("no qemu-img")
	}
	td := t.TempDir()
	dev := filepath.Join(td, "dev")
	require.NoError(t, os.WriteFile(dev, make([]byte, 2<<20), 0o600))
	image := filepath.Join(td, "image.qcow2")
	backed := filepath.Join(td, "backed.qcow2")
	run := func(args ...string) {
		b, err := exec.Command("qemu-img", args...).CombinedOutput()
		require.NoError(t, err, string(b))
	}
	run("create", "-f", "qcow2", image, "1M")
	run("create", "-f", "qcow2", "-F", "raw", "-b", dev, backed, "1M")

	r := &T{Image: image}
	r.SetLoggerForTest(plog.NewDefaultLogger())
	assert.NoError(t, r.writeImage(context.Background(), dev))
	fi, err := os.Stat(dev)
	require.NoError(t, err)
	assert.Equal(t, int64(2<<20), fi.Size(), "the device keeps its size")

	r.Image = backed
	assert.ErrorContains(t, r.writeImage(context.Background(), dev), "backing file")
}

// A downloaded image is parsed as nobody, who is lent the device for the time
// of the write only.
func TestWriteImageDownloaded(t *testing.T) {
	if _, err := exec.LookPath("qemu-img"); err != nil {
		t.Skip("no qemu-img")
	}
	if os.Getuid() != 0 {
		t.Skip("needs root")
	}
	allowLocalURLs(t)
	td := t.TempDir()
	// The device of a node is in /dev, which is open to all.
	require.NoError(t, os.Chmod(filepath.Dir(td), 0o755))
	require.NoError(t, os.Chmod(td, 0o755))
	dev := filepath.Join(td, "dev")
	require.NoError(t, os.WriteFile(dev, make([]byte, 2<<20), 0o600))
	image := filepath.Join(td, "image.qcow2")
	b, err := exec.Command("qemu-img", "create", "-f", "qcow2", image, "1M").CombinedOutput()
	require.NoError(t, err, string(b))
	srv := httptest.NewServer(http.FileServer(http.Dir(td)))
	defer srv.Close()

	r := &T{Image: srv.URL + "/image.qcow2"}
	r.SetLoggerForTest(plog.NewDefaultLogger())
	require.NoError(t, r.writeImage(context.Background(), dev))

	var st syscall.Stat_t
	require.NoError(t, syscall.Stat(dev, &st))
	assert.Equal(t, uint32(0), st.Uid, "the device is taken back")
}

func TestCDROMFile(t *testing.T) {
	allowLocalURLs(t)
	saved := cdromDir
	cdromDir = t.TempDir()
	defer func() { cdromDir = saved }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("installer"))
	}))
	defer srv.Close()
	dev := filepath.Join(t.TempDir(), "dev")
	require.NoError(t, os.WriteFile(dev, make([]byte, 16), 0o600))

	r := &T{Name: "vm1", CDROM: "/srv/i.iso"}
	p, err := r.cdromFile(context.Background(), dev)
	require.NoError(t, err)
	assert.Equal(t, "/srv/i.iso", p, "a path is used as it is")

	r.CDROM = srv.URL + "/i.iso"
	p, err = r.cdromFile(context.Background(), dev)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cdromDir, "vm1.iso"), p)
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), fi.Mode().Perm(), "qemu reads it")

	require.NoError(t, r.removeCDROM())
	require.NoError(t, r.removeCDROM(), "what is gone is removed")
	assert.NoFileExists(t, p)
}

func TestCopyVolumeFile(t *testing.T) {
	head := t.TempDir()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(head, "a.img"), []byte("image"), 0o644))

	p, err := copyVolumeFile(head, filepath.Join(head, "a.img"), dir, 5)
	require.NoError(t, err)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "image", string(b))

	_, err = copyVolumeFile(head, filepath.Join(head, "a.img"), dir, 4)
	assert.ErrorContains(t, err, "larger than")
	_, err = copyVolumeFile(head, head, dir, 5)
	assert.ErrorContains(t, err, "no regular file")

	// A file in a directory of the volume, as volume#2:/isos/debian.iso.
	require.NoError(t, os.MkdirAll(filepath.Join(head, "isos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(head, "isos", "debian.iso"), []byte("iso"), 0o644))
	p, err = copyVolumeFile(head, filepath.Join(head, "isos", "debian.iso"), dir, 5)
	require.NoError(t, err)
	b, err = os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "iso", string(b))
}

// What a volume holds is its user's: a link leading out of it is no way to the
// files of the node, which root would copy to the disk of the guest.
func TestCopyVolumeFileStaysInTheVolume(t *testing.T) {
	head := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(secret, filepath.Join(head, "a.img")))
	_, err := copyVolumeFile(head, filepath.Join(head, "a.img"), t.TempDir(), 1024)
	assert.Error(t, err)
}

// The definition of a domain is shared by the nodes: a device mapper or a loop
// minor would name another disk on the next one, or at the next boot.
func TestStableDevpath(t *testing.T) {
	saved := sysBlock
	sysBlock = t.TempDir()
	defer func() { sysBlock = saved }()
	write := func(p, s string) {
		p = filepath.Join(sysBlock, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(s+"\n"), 0o644))
	}
	write("dm-17/dm/name", "36001405726ac48477444c949936811ee")
	write("loop2/loop/backing_file", "/var/lib/opensvc/pool/loop/a.img")

	assert.Equal(t, "/dev/mapper/36001405726ac48477444c949936811ee", stableDevpath("/dev/dm-17"))
	assert.Equal(t, "/var/lib/opensvc/pool/loop/a.img", stableDevpath("/dev/loop2"))
	assert.Equal(t, "/dev/zvol/tank/a", stableDevpath("/dev/zvol/tank/a"), "stable already")
	assert.Equal(t, "/dev/drbd3", stableDevpath("/dev/drbd3"))
	assert.Equal(t, "/dev/dm-99", stableDevpath("/dev/dm-99"), "nothing better known")
}

// A cdrom keyword naming a path is the file of the operator, even at the path
// the copy of a download would have: what removes that copy leaves it.
func TestRemoveCDROMLeavesTheFileOfTheOperator(t *testing.T) {
	saved := cdromDir
	cdromDir = t.TempDir()
	defer func() { cdromDir = saved }()
	r := &T{}
	r.Name = "vm1"
	require.NoError(t, os.WriteFile(r.cdromPath(), []byte("iso"), 0o644))

	r.CDROM = r.cdromPath()
	require.NoError(t, r.removeCDROM())
	assert.FileExists(t, r.cdromPath())

	r.CDROM = "https://example.com/debian.iso"
	require.NoError(t, r.removeCDROM())
	assert.NoFileExists(t, r.cdromPath())
}
