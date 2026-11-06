package uri

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/httppolicy"
)

// serveOn starts a server answering with h on the loopback address given.
func serveOn(t *testing.T, addr string, h http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp", addr+":0")
	require.NoError(t, err)
	s := httptest.NewUnstartedServer(h)
	s.Listener = l
	s.Start()
	t.Cleanup(s.Close)
	return s
}

func get(t *testing.T, policy *httppolicy.T, rawURL string, redirects bool) (*http.Response, error) {
	t.Helper()
	ip, port, err := policy.Check(rawURL)
	require.NoError(t, err)
	c, err := pinnedClient(policy, rawURL, ip, port, redirects)
	require.NoError(t, err)
	return c.Get(rawURL)
}

// A redirect of a url the policy accepts is followed to a host the url lists
// do not name, the address of each hop checked: a release download is served
// by a storage its url does not say.
func TestRedirectFollowedToAnAcceptedAddress(t *testing.T) {
	storage := serveOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "asset")
	}))
	release := serveOn(t, "127.0.0.1", http.RedirectHandler(storage.URL+"/asset", http.StatusFound))
	policy := httppolicy.New([]string{release.URL + "/*"}, []string{"*"}, []string{"127.0.0.1/32"}, []string{"127.0.0.0/8"})

	resp, err := get(t, policy, release.URL+"/download", true)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "asset", string(b))

	resp, err = get(t, policy, release.URL+"/download", false)
	require.NoError(t, err, "redirects disabled: the redirect is the answer")
	resp.Body.Close()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
}

// A redirect to an address the cidr lists block is refused, whatever url
// redirects to it, and a redirect loop stops.
func TestRedirectRefused(t *testing.T) {
	blocked := serveOn(t, "127.0.0.2", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "internal")
	}))
	release := serveOn(t, "127.0.0.1", http.RedirectHandler(blocked.URL+"/secret", http.StatusFound))
	policy := httppolicy.New([]string{release.URL + "/*"}, []string{"*"}, []string{"127.0.0.1/32"}, []string{"127.0.0.0/8"})
	_, err := get(t, policy, release.URL+"/download", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is blocked")

	var loop *httptest.Server
	loop = serveOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, loop.URL+"/again", http.StatusFound)
	}))
	policy = httppolicy.New([]string{loop.URL + "/*"}, []string{"*"}, []string{"127.0.0.1/32"}, []string{"127.0.0.0/8"})
	_, err = get(t, policy, loop.URL+"/start", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stopped after")
}

// A request carrying data does not follow a redirect: a 307 would send its
// body again, to the host the redirect names.
func TestRedirectOfARequestWithDataRefused(t *testing.T) {
	var got []byte
	other := serveOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
	}))
	api := serveOn(t, "127.0.0.1", http.RedirectHandler(other.URL+"/collect", http.StatusTemporaryRedirect))
	policy := httppolicy.New([]string{api.URL + "/*"}, []string{"*"}, []string{"127.0.0.1/32"}, []string{"127.0.0.0/8"})
	ip, port, err := policy.Check(api.URL + "/api")
	require.NoError(t, err)
	c, err := pinnedClient(policy, api.URL+"/api", ip, port, true)
	require.NoError(t, err)
	_, err = c.Post(api.URL+"/api", "application/json", strings.NewReader(`{"secret":"x"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only a download follows a redirect")
	assert.Empty(t, got, "the body is not sent to the redirect target")
}
