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
