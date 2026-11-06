package datarecv

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/volfile"
)

// A line naming a source and no datastore fetches a file, one naming a
// datastore installs its keys, and any other is a directory: the line of the
// web ui of oc3 made a directory of the name of the file before.
func TestParseFetchLine(t *testing.T) {
	recv := &DataRecv{Install: []string{
		"/web/html/index.html.gz", "required", "source", "https://github.com/opensvc/oc3-frontend/releases/latest/download/index.html.gz",
		"/bin/tool", "source", "https://example.com/tool", "mode", "0755", "user", "101", "group", "102", "signal", "HUP:container#1", "sha256", strings.Repeat("A", 64),
		"/etc/", "mode", "0750",
		"/etc/app.conf", "from", "./cfg/app", "key", "app.conf",
	}}
	recv.Perm = nil
	dirs, files, _ := recv.parseInstallLines("/head", naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"})
	require.Empty(t, recv.sourceErrs)
	require.Len(t, dirs, 1)
	assert.Equal(t, "/etc/", dirs[0].Path)
	require.Len(t, files, 1)
	require.Len(t, recv.fetches, 2)

	fd := recv.fetches[0]
	assert.Equal(t, "/web/html/index.html.gz", fd.Path)
	assert.Equal(t, "https://github.com/opensvc/oc3-frontend/releases/latest/download/index.html.gz", fd.URL)
	assert.True(t, fd.Required)
	assert.Equal(t, os.FileMode(0644), fd.Perm, "the default mode of a cfg key")
	assert.Equal(t, "url:"+fd.URL+":/web/html/index.html.gz", fd.lineID())

	fd = recv.fetches[1]
	assert.Equal(t, os.FileMode(0755), fd.Perm)
	assert.Equal(t, "101", fd.User)
	assert.Equal(t, "102", fd.Group)
	assert.Equal(t, strings.Repeat("a", 64), fd.SHA256)
	assert.False(t, fd.Required)
	assert.Len(t, fd.Signals.Routes(), 1)

	l, err := InstallKeySources(recv.Install, "dev")
	require.NoError(t, err)
	assert.Empty(t, l, "a file fetched seeds no key")
}

// A line fetching a file fetches it from an https url, root or not, names a
// file, and is no key nor template: the install and the validation of the
// configuration refuse the others, saying why.
func TestFetchLineRefusals(t *testing.T) {
	for _, tc := range []struct {
		line string
		want string
	}{
		{"/web/ source https://example.com/index.html", "not a directory"},
		{"/ source https://example.com/index.html", "not a directory"},
		{"/web/index.html source http://example.com/index.html", "https:// url"},
		{"/web/index.html source /srv/index.html", "https:// url"},
		{"/web/index.html source random", "https:// url"},
		{"/web/index.html source https:/etc/shadow", "https:// url"},
		{"/web/index.html source", "https:// url"},
		{"/web/index.html key index.html source https://example.com/index.html", "no key"},
		{"/web/index.html source https://example.com/index.html template", "template"},
		{"/web/index.html source https://example.com/index.html sha256 abc", "64 hexadecimal digits"},
	} {
		err := ValidateInstall(tc.line)
		require.Error(t, err, tc.line)
		assert.Contains(t, err.Error(), tc.want, tc.line)

		recv := &DataRecv{Install: strings.Fields(tc.line)}
		recv.parseInstallLines("/head", naming.Path{Namespace: "dev", Kind: naming.KindSvc, Name: "web"})
		require.Len(t, recv.sourceErrs, 1, tc.line)
		assert.Empty(t, recv.fetches, tc.line)
		_, err = InstallKeySources(strings.Fields(tc.line), "dev")
		assert.Error(t, err, tc.line)
	}
	assert.NoError(t, ValidateInstall("/web/index.html source https://example.com/index.html sha256 "+strings.Repeat("0", 64)))
}

// fetchServer serves the files of a test, counting the fetches of each.
type fetchServer struct {
	*httptest.Server
	mu    sync.Mutex
	files map[string]string
	count map[string]int
}

func newFetchServer(t *testing.T) *fetchServer {
	s := &fetchServer{files: make(map[string]string), count: make(map[string]int)}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		body, ok := s.files[r.URL.Path]
		s.count[r.URL.Path]++
		s.mu.Unlock()
		switch {
		case !ok:
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/streamed/"):
			// No length announced.
			w.WriteHeader(http.StatusOK)
			for _, c := range []byte(body) {
				_, _ = w.Write([]byte{c})
				w.(http.Flusher).Flush()
			}
		default:
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(s.Close)
	formerFetch, formerMax := fetchHTTP, installSizeMax
	t.Cleanup(func() { fetchHTTP, installSizeMax = formerFetch, formerMax })
	fetchHTTP = func(ctx context.Context, u string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		return s.Client().Do(req)
	}
	return s
}

func (s *fetchServer) set(path, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = body
}

func (s *fetchServer) fetches(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count[path]
}

func setupFetch(t *testing.T) object.Core {
	t.Helper()
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)
	p := naming.Path{Name: "oc3", Kind: naming.KindSvc, Namespace: naming.NsRoot}
	require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"), 0600))
	o, err := object.NewCore(p)
	require.NoError(t, err)
	return o
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// A file is fetched when it is missing, or its url changed, and a start
// finding the file of the url fetches nothing. A file deleted is fetched
// again, a url changed for a failing one leaves the file of the former url,
// and a line taken away takes its file with it.
func TestInstallFetch(t *testing.T) {
	o := setupFetch(t)
	srv := newFetchServer(t)
	srv.set("/v1/index.html.gz", "v1")
	srv.set("/v2/index.html.gz", "v2")
	head := t.TempDir()
	dst := filepath.Join(head, "web/html/index.html.gz")
	line := func(version string) []string {
		return []string{"/web/html/index.html.gz", "source", srv.URL + "/" + version + "/index.html.gz"}
	}
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: line("v1")}
	install := func() bool {
		t.Helper()
		changed, err := recv.install(context.Background())
		require.NoError(t, err)
		return changed
	}

	assert.True(t, install())
	assert.Equal(t, "v1", readFile(t, dst))
	assert.Equal(t, 1, srv.fetches("/v1/index.html.gz"))
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	if m, ok, err := volfile.Installed(head, "web/html", "index.html.gz"); err == nil && ok {
		assert.Equal(t, srv.URL+"/v1/index.html.gz", m.From)
		assert.Empty(t, m.Key)
	}

	assert.False(t, install(), "the file of the url is there")
	assert.Equal(t, 1, srv.fetches("/v1/index.html.gz"), "no fetch at a start finding the file")

	// A filesystem keeping no mark: the record says the line fetched it.
	_ = unix.Removexattr(dst, volfile.InstallAttr)
	assert.False(t, install())
	assert.Equal(t, 1, srv.fetches("/v1/index.html.gz"))

	require.NoError(t, os.Remove(dst))
	assert.True(t, install(), "a file deleted is fetched again")
	assert.Equal(t, 2, srv.fetches("/v1/index.html.gz"))

	recv.Install = line("v3")
	assert.False(t, install(), "a url failing, the line not required, warns")
	assert.Equal(t, "v1", readFile(t, dst), "the file of the former url stays")

	recv.Install = line("v2")
	assert.True(t, install())
	assert.Equal(t, "v2", readFile(t, dst), "a url changed is fetched")
	files := readInstallFiles(recv.installFilesFile())
	assert.Len(t, files, 1, "the line of the former url is no longer recorded")
	assert.Contains(t, files, "url:"+srv.URL+"/v2/index.html.gz:/web/html/index.html.gz")

	recv.Install = []string{"/web/", "mode", "0755"}
	install()
	assert.NoFileExists(t, dst, "a line taken away takes its file with it")

	l, err := os.ReadDir(head)
	require.NoError(t, err)
	for _, e := range l {
		assert.False(t, strings.HasPrefix(e.Name(), ".om-upload-"), "nothing left aside")
	}
}

// A required fetch failing fails the install, naming the url. The empty
// directory the line made, before it fetched, is replaced by the file.
func TestInstallFetchRequired(t *testing.T) {
	o := setupFetch(t)
	srv := newFetchServer(t)
	head := t.TempDir()
	url := srv.URL + "/missing/index.html.gz"
	require.NoError(t, os.MkdirAll(filepath.Join(head, "web/html/index.html.gz"), 0755))
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: []string{"/web/html/index.html.gz", "required", "source", url}}
	_, err := recv.install(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), url)
	assert.Contains(t, err.Error(), "HTTP 404")

	srv.set("/missing/index.html.gz", "found")
	_, err = recv.install(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "found", readFile(t, filepath.Join(head, "web/html/index.html.gz")))
}

// A fetch holds node.max_install_size at most, announced or sent, and a
// body of another sha256 than the line names is refused: the file fetched
// before stays, and nothing is left aside.
func TestInstallFetchRefused(t *testing.T) {
	o := setupFetch(t)
	srv := newFetchServer(t)
	installSizeMax = func() (int64, error) { return 4, nil }
	srv.set("/announced", "too large")
	srv.set("/streamed/a", "too large")
	srv.set("/ok", "fits")
	sum := sha256.Sum256([]byte("fits"))
	good := hex.EncodeToString(sum[:])
	bad := strings.Repeat("0", 64)
	head := t.TempDir()
	dst := filepath.Join(head, "f")
	require.NoError(t, os.WriteFile(dst, []byte("before"), 0644))

	for _, tc := range []struct {
		path string
		more []string
		want []string
	}{
		{"/announced", nil, []string{"announced", "node.max_install_size"}},
		{"/streamed/a", nil, []string{"more than 4.00B sent", "node.max_install_size"}},
		{"/ok", []string{"sha256", bad}, []string{"sha256 " + bad + " expected, got " + good}},
	} {
		url := srv.URL + tc.path
		recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: append([]string{"/f", "required", "source", url}, tc.more...)}
		_, err := recv.install(context.Background())
		require.Error(t, err, tc.path)
		assert.Contains(t, err.Error(), url, tc.path)
		for _, s := range tc.want {
			assert.Contains(t, err.Error(), s, tc.path)
		}
		assert.Equal(t, "before", readFile(t, dst), tc.path)
		l, err := os.ReadDir(head)
		require.NoError(t, err)
		assert.Len(t, l, 1, "nothing left aside")
	}

	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: []string{"/f", "source", srv.URL + "/ok", "sha256", good}}
	_, err := recv.install(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "fits", readFile(t, dst))
}

// The refresh a write of a datastore runs fetches nothing, and leaves the
// file of a line fetching it, which is no line of the datastore.
func TestInstallFromDatastoreKeepsTheFilesFetched(t *testing.T) {
	o := setupFetch(t)
	srv := newFetchServer(t)
	srv.set("/index.html.gz", "ui")
	cfg := naming.Path{Name: "oc3", Kind: naming.KindCfg, Namespace: naming.NsRoot}
	ds, err := object.NewDataStore(cfg)
	require.NoError(t, err)
	require.NoError(t, ds.AddKey("config.yaml", []byte("a")))

	head := t.TempDir()
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: []string{
		"/web/html/index.html.gz", "source", srv.URL + "/index.html.gz",
		"/config.yaml", "from", "./cfg/oc3",
	}}
	_, err = recv.install(context.Background())
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(head, "web/html/index.html.gz"))

	require.NoError(t, ds.AddKey("other", []byte("b")))
	_, err = recv.InstallFromDatastore(context.Background(), ds)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(head, "web/html/index.html.gz"))
	assert.Equal(t, 1, srv.fetches("/index.html.gz"))
}

// A file written over since it was fetched is fetched again, rather than
// served as the content of its url: checked against the digest of its mark,
// against the one this node recorded on a volume keeping no mark, and
// against the sha256 the line names.
func TestInstallFetchChangedSince(t *testing.T) {
	o := setupFetch(t)
	srv := newFetchServer(t)
	srv.set("/v1/index.html.gz", "v1")
	head := t.TempDir()
	dst := filepath.Join(head, "web/html/index.html.gz")
	url := srv.URL + "/v1/index.html.gz"
	recv := &DataRecv{to: headReceiver{o: o, head: head}, Install: []string{"/web/html/index.html.gz", "source", url}}
	install := func() bool {
		t.Helper()
		changed, err := recv.install(context.Background())
		require.NoError(t, err)
		return changed
	}
	// writeOver changes the content, keeping the file, its mark included.
	writeOver := func() {
		t.Helper()
		f, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0)
		require.NoError(t, err)
		_, err = f.WriteString("tampered")
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	require.True(t, install())
	writeOver()
	assert.True(t, install(), "the mark's digest tells the file changed")
	assert.Equal(t, "v1", readFile(t, dst))
	assert.Equal(t, 2, srv.fetches("/v1/index.html.gz"))
	assert.False(t, install(), "the content of the url is kept")

	writeOver()
	_ = unix.Removexattr(dst, volfile.InstallAttr)
	assert.True(t, install(), "with no mark, the digest recorded tells the file changed")
	assert.Equal(t, "v1", readFile(t, dst))

	sum := sha256.Sum256([]byte("v1"))
	recv.Install = []string{"/web/html/index.html.gz", "source", url, "sha256", hex.EncodeToString(sum[:])}
	writeOver()
	assert.True(t, install(), "the sha256 of the line tells the file changed")
	assert.Equal(t, "v1", readFile(t, dst))
	assert.Equal(t, 4, srv.fetches("/v1/index.html.gz"))
}
