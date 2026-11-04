package pkgupdate

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeManager is a deb package manager whose installed packages and offers
// are given, and whose commands are recorded and run as true.
type fakeManager struct {
	installed map[string]string
	offered   map[string]bool
	ran       [][]string
}

func (t *fakeManager) Name() string            { return "fake" }
func (t *fakeManager) Format() string          { return "deb" }
func (t *fakeManager) Env() []string           { return nil }
func (t *fakeManager) Compare(a, b string) int { return CompareDeb(a, b) }
func (t *fakeManager) Installed(name string) (string, error) {
	return t.installed[name], nil
}
func (t *fakeManager) Offered(name string) (bool, error) { return t.offered[name], nil }
func (t *fakeManager) Refresh() []string                 { return t.record("refresh") }
func (t *fakeManager) Upgrade(names []string) []string {
	return t.record(append([]string{"upgrade"}, names...)...)
}
func (t *fakeManager) Install(files []string) []string {
	// The files are gone once the update returns: their names and content now.
	for _, f := range files {
		b, _ := os.ReadFile(f)
		t.ran = append(t.ran, []string{"content", filepath.Base(f), string(b)})
	}
	return t.record(append([]string{"install"}, files...)...)
}

func (t *fakeManager) record(argv ...string) []string {
	t.ran = append(t.ran, argv)
	return []string{"true"}
}

func init() {
	detachedArgv = func(argv []string) []string { return argv }
}

// aptRepo serves an apt repository holding the packages files, by name.
func aptRepo(t *testing.T, files map[string]string, sums map[string]string) *httptest.Server {
	t.Helper()
	var index strings.Builder
	for name, content := range files {
		pkg, version, _ := strings.Cut(strings.TrimSuffix(name, ".deb"), "_")
		sum := sha256.Sum256([]byte(content))
		s := hex.EncodeToString(sum[:])
		if v, ok := sums[name]; ok {
			s = v
		}
		fmt.Fprintf(&index, "Package: %s\nVersion: %s\nArchitecture: %s\nFilename: pool/%s\nSHA256: %s\n\n", pkg, version, DebArch(), name, s)
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(index.String()))
	_ = w.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/apt/ubuntu/dists/prod-opensvc-v3-resolute/main/binary-"+DebArch()+"/Packages.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(gz.Bytes())
	})
	for name, content := range files {
		content := content
		mux.HandleFunc("/apt/ubuntu/pool/"+name, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(content))
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newUpdate(t *testing.T, mgr Manager, base string) (*T, *bytes.Buffer) {
	t.Helper()
	osRelease := filepath.Join(t.TempDir(), "os-release")
	require.NoError(t, os.WriteFile(osRelease, []byte("ID=ubuntu\nVERSION_CODENAME=resolute\n"), 0o644))
	var out bytes.Buffer
	return &T{Base: base, Manager: mgr, Out: &out, OSRelease: osRelease, TmpDir: t.TempDir()}, &out
}

func TestRunConfiguredRepository(t *testing.T) {
	mgr := &fakeManager{
		installed: map[string]string{"opensvc-server": "3.0.0~rc55", "opensvc-webapp": "1.0.0~rc9"},
		offered:   map[string]bool{"opensvc-server": true},
	}
	u, out := newUpdate(t, mgr, "http://127.0.0.1:1")
	require.NoError(t, u.Run(context.Background()))
	assert.Equal(t, [][]string{{"refresh"}, {"upgrade", "opensvc-server"}}, mgr.ran,
		"the packages a configured repository offers are upgraded from it, the client not installed is not")
	assert.Contains(t, out.String(), "opensvc-webapp 1.0.0~rc9 installed, offered by no configured repository")
}

func TestRunDownload(t *testing.T) {
	srv := aptRepo(t, map[string]string{
		"opensvc-server_3.0.0~rc7.deb":  "rc7",
		"opensvc-server_3.0.0~rc56.deb": "rc56",
		"opensvc-client_3.0.0~rc55.deb": "rc55",
	}, nil)
	mgr := &fakeManager{installed: map[string]string{"opensvc-server": "3.0.0~rc55", "opensvc-client": "3.0.0~rc55"}}
	u, out := newUpdate(t, mgr, srv.URL)
	require.NoError(t, u.Run(context.Background()))
	require.Len(t, mgr.ran, 3)
	assert.Equal(t, []string{"refresh"}, mgr.ran[0])
	assert.Equal(t, []string{"content", "opensvc-server_3.0.0~rc56.deb", "rc56"}, mgr.ran[1],
		"the newest version is installed, and only the packages newer than the installed ones")
	assert.Equal(t, "install", mgr.ran[2][0])
	assert.Len(t, mgr.ran[2], 2)
	assert.Contains(t, out.String(), "opensvc-client 3.0.0~rc55 installed, up to date")
}

func TestRunDownloadBadSum(t *testing.T) {
	srv := aptRepo(t, map[string]string{"opensvc-server_3.0.0~rc56.deb": "rc56"},
		map[string]string{"opensvc-server_3.0.0~rc56.deb": strings.Repeat("0", 64)})
	mgr := &fakeManager{installed: map[string]string{"opensvc-server": "3.0.0~rc55"}}
	u, _ := newUpdate(t, mgr, srv.URL)
	err := u.Run(context.Background())
	require.ErrorContains(t, err, "sha256")
	assert.Equal(t, [][]string{{"refresh"}}, mgr.ran, "a package not matching its sha256 is not installed")
}

func TestRunDryRun(t *testing.T) {
	srv := aptRepo(t, map[string]string{"opensvc-server_3.0.0~rc56.deb": "rc56"}, nil)
	mgr := &fakeManager{installed: map[string]string{"opensvc-server": "3.0.0~rc55"}}
	u, out := newUpdate(t, mgr, srv.URL)
	u.DryRun = true
	require.NoError(t, u.Run(context.Background()))
	assert.Empty(t, mgr.ran, "a dry run refreshes, downloads and installs nothing")
	assert.Contains(t, out.String(), "opensvc-server 3.0.0~rc55 installed, upgrade to 3.0.0~rc56")
}

func TestRunNothingInstalled(t *testing.T) {
	u, _ := newUpdate(t, &fakeManager{}, "http://127.0.0.1:1")
	assert.ErrorContains(t, u.Run(context.Background()), "none of the packages")
}

func TestParseManagerOutputs(t *testing.T) {
	assert.True(t, parseMadison(` opensvc-server | 3.0.0~rc56 | https://packages.opensvc.com/apt/ubuntu prod-opensvc-v3-resolute/main amd64 Packages
`, "opensvc-server"))
	assert.False(t, parseMadison("", "opensvc-server"))

	assert.True(t, parseYumList(`Installed Packages
opensvc-server.x86_64     3.0.0~rc55-1.el7     installed
Available Packages
opensvc-server.x86_64     3.0.0~rc56-1.el7     opensvc
`, "opensvc-server"))
	assert.False(t, parseYumList(`Installed Packages
opensvc-server.x86_64     3.0.0~rc55-1.el7     @opensvc
`, "opensvc-server"), "the installed package says nothing of the repositories")

	assert.True(t, parseZypperSearch(`S  | Name           | Type    | Version    | Arch   | Repository
---+----------------+---------+------------+--------+-----------
v  | opensvc-server | package | 3.0.0~rc56 | x86_64 | opensvc
i+ | opensvc-server | package | 3.0.0~rc55 | x86_64 | (System Packages)
`, "opensvc-server"))
	assert.False(t, parseZypperSearch(`S  | Name           | Type    | Version    | Arch   | Repository
---+----------------+---------+------------+--------+------------------
i+ | opensvc-server | package | 3.0.0~rc55 | x86_64 | (System Packages)
`, "opensvc-server"))
}
