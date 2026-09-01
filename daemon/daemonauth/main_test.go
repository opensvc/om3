package daemonauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	// fakeStrategy accepts or refuses every request, so that the union
	// can be tested without credentials.
	fakeStrategy struct {
		info *Info
		err  error
	}

	// fakeUserDB answers for the usr objects of one cluster.
	fakeUserDB struct {
		grants   map[string][]string
		password map[string]string
	}
)

func (s fakeStrategy) Authenticate(context.Context, *http.Request) (*Info, error) {
	return s.info, s.err
}

func (db fakeUserDB) GrantsFromUsername(username string) ([]string, error) {
	grants, ok := db.grants[username]
	if !ok {
		return nil, fmt.Errorf("username '%s' does not exist", username)
	}
	return grants, nil
}

func (db fakeUserDB) GrantsFromUsernameAndPassword(username, password string) ([]string, error) {
	if db.password[username] != password || password == "" {
		return nil, fmt.Errorf("wrong password")
	}
	return db.GrantsFromUsername(username)
}

// testAuthOption implements the interfaces initX509 asks of the value it
// is configured with.
type testAuthOption struct {
	caCertFile string
}

func (o testAuthOption) X509CACertFile() string { return o.caCertFile }

func (o testAuthOption) GrantsFromUsername(username string) ([]string, error) {
	return testUserDB().GrantsFromUsername(username)
}

func testUserDB() fakeUserDB {
	return fakeUserDB{
		grants:   map[string][]string{"alice": {"admin:test"}},
		password: map[string]string{"alice": "secret"},
	}
}

func TestUnionStopsAtTheFirstStrategyThatAccepts(t *testing.T) {
	accepted := &Info{Username: "alice"}
	u := Union{
		fakeStrategy{err: fmt.Errorf("no basic auth header")},
		fakeStrategy{info: accepted},
		fakeStrategy{info: &Info{Username: "someone else"}},
	}
	r, err := http.NewRequest(http.MethodGet, "https://localhost/", nil)
	require.NoError(t, err)

	info, err := u.AuthenticateRequest(r)
	require.NoError(t, err)
	assert.Same(t, accepted, info)
}

func TestUnionReportsWhatEveryStrategySaid(t *testing.T) {
	u := Union{
		fakeStrategy{err: fmt.Errorf("no basic auth header")},
		fakeStrategy{err: fmt.Errorf("no bearer token")},
	}
	r, err := http.NewRequest(http.MethodGet, "https://localhost/", nil)
	require.NoError(t, err)

	_, err = u.AuthenticateRequest(r)
	require.Error(t, err)
	assert.ErrorContains(t, err, "no basic auth header")
	assert.ErrorContains(t, err, "no bearer token")
}

func TestEmptyUnionAuthenticatesNobody(t *testing.T) {
	r, err := http.NewRequest(http.MethodGet, "https://localhost/", nil)
	require.NoError(t, err)
	_, err = Union{}.AuthenticateRequest(r)
	assert.Error(t, err)
}

func basicRequest(t *testing.T, username, password string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "https://localhost/api/whoami", nil)
	require.NoError(t, err)
	if username != "" {
		r.SetBasicAuth(username, password)
	}
	return r
}

func TestBasicUserStrategy(t *testing.T) {
	for name, tc := range map[string]struct {
		username string
		password string
		grants   []string
	}{
		"a usr object and its password": {"alice", "secret", []string{"admin:test"}},
		"a wrong password":              {"alice", "guess", nil},
		"an empty password":             {"alice", "", nil},
		"an unknown user":               {"bob", "secret", nil},
		"no credentials at all":         {"", "", nil},
	} {
		t.Run(name, func(t *testing.T) {
			authCache = newCache()
			s := &basicUserStrategy{userDB: testUserDB()}
			info, err := s.Authenticate(context.Background(), basicRequest(t, tc.username, tc.password))
			if tc.grants == nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.username, info.Username)
			assert.Equal(t, StrategyUser, info.Strategy)
			assert.Equal(t, tc.grants, info.Grants)
		})
	}
}

func TestBasicUserStrategyCachesPerCredential(t *testing.T) {
	// The cache is keyed by the credential, not by the user: a request
	// with the wrong password must not be answered by the entry a request
	// with the right one left behind.
	authCache = newCache()
	s := &basicUserStrategy{userDB: testUserDB()}
	_, err := s.Authenticate(context.Background(), basicRequest(t, "alice", "secret"))
	require.NoError(t, err)
	_, err = s.Authenticate(context.Background(), basicRequest(t, "alice", "guess"))
	assert.Error(t, err)
}

// testCAPEM returns the pem encoding of a certificate, as the
// certificate_chain key of a ca sec object holds it.
func testCAPEM(t *testing.T, cert *x509.Certificate) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

// testCA returns a certificate authority and a function signing client
// certificates with it.
func testCA(t *testing.T) (*x509.Certificate, func(cn string) *x509.Certificate) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	sign := func(cn string) *x509.Certificate {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		template := &x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}
		der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
		require.NoError(t, err)
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		return cert
	}
	return ca, sign
}

func tlsRequest(t *testing.T, certs ...*x509.Certificate) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "https://localhost/api/whoami", nil)
	require.NoError(t, err)
	if len(certs) > 0 {
		r.TLS = &tls.ConnectionState{PeerCertificates: certs}
	}
	return r
}

func newTestX509Strategy(t *testing.T, ca *x509.Certificate) *X509Strategy {
	t.Helper()
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return &X509Strategy{
		verifyOptions: x509.VerifyOptions{
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			Roots:     roots,
		},
		userDB: testUserDB(),
	}
}

func TestX509StrategyAcceptsACertificateOfTheClusterCA(t *testing.T) {
	authCache = newCache()
	ca, sign := testCA(t)
	s := newTestX509Strategy(t, ca)

	info, err := s.Authenticate(context.Background(), tlsRequest(t, sign("alice")))
	require.NoError(t, err)
	assert.Equal(t, "alice", info.Username)
	assert.Equal(t, StrategyX509, info.Strategy)
	assert.Equal(t, []string{"admin:test"}, info.Grants)
}

func TestX509StrategyRefusals(t *testing.T) {
	authCache = newCache()
	ca, sign := testCA(t)
	_, signOther := testCA(t)
	s := newTestX509Strategy(t, ca)

	t.Run("a request with no client certificate", func(t *testing.T) {
		_, err := s.Authenticate(context.Background(), tlsRequest(t))
		assert.Error(t, err)
	})
	t.Run("a certificate of another ca", func(t *testing.T) {
		_, err := s.Authenticate(context.Background(), tlsRequest(t, signOther("alice")))
		assert.Error(t, err)
	})
	t.Run("a certificate naming no one", func(t *testing.T) {
		_, err := s.Authenticate(context.Background(), tlsRequest(t, sign("")))
		assert.Error(t, err)
	})
	t.Run("a certificate naming an unknown user", func(t *testing.T) {
		_, err := s.Authenticate(context.Background(), tlsRequest(t, sign("bob")))
		assert.Error(t, err)
	})
}

// TestX509StrategyTrustsEveryAuthorityInTheCAFile covers the ca file the
// listener writes: the chain of the cluster ca, then the chain of each ca
// of the cluster ca_sec_paths. A client signed by any of them is a client
// this cluster was told to trust.
func TestX509StrategyTrustsEveryAuthorityInTheCAFile(t *testing.T) {
	authCache = newCache()
	clusterCA, signCluster := testCA(t)
	secondCA, signSecond := testCA(t)

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca_certificates")
	pems := append(testCAPEM(t, clusterCA), testCAPEM(t, secondCA)...)
	require.NoError(t, os.WriteFile(caFile, pems, 0600))

	_, strategy, err := initX509(context.Background(), testAuthOption{caCertFile: caFile})
	require.NoError(t, err)
	require.NotNil(t, strategy)

	for name, cert := range map[string]*x509.Certificate{
		"the cluster ca": signCluster("alice"),
		"a secondary ca": signSecond("alice"),
	} {
		t.Run(name, func(t *testing.T) {
			info, err := strategy.Authenticate(context.Background(), tlsRequest(t, cert))
			require.NoError(t, err)
			assert.Equal(t, "alice", info.Username)
		})
	}

	_, signUnknown := testCA(t)
	_, err = strategy.Authenticate(context.Background(), tlsRequest(t, signUnknown("alice")))
	assert.Error(t, err, "an authority the cluster was not told about stays untrusted")
}

func TestInitX509WithoutACertificate(t *testing.T) {
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca_certificates")
	require.NoError(t, os.WriteFile(caFile, []byte("not a certificate\n"), 0600))
	_, _, err := initX509(context.Background(), testAuthOption{caCertFile: caFile})
	assert.Error(t, err)
}
