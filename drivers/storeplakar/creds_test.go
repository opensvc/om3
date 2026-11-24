package storeplakar

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/plog"
)

const (
	s3Secret  = "s3-secret-not-to-be-seen"
	httpToken = "token-not-to-be-seen"
)

// credsEnv is a test environment whose sec holds the credentials of every
// scheme, with the ssh of the node faked: no test of the fake runs it.
type credsEnv struct {
	testEnv
	key     []byte
	hostKey string
}

func newCredsEnv(t *testing.T) credsEnv {
	t.Helper()
	e := credsEnv{testEnv: newTestEnv(t)}
	sshPath = func() (string, error) { return "/usr/bin/ssh", nil }
	t.Cleanup(func() { sshPath = func() (string, error) { return exec.LookPath("ssh") } })
	e.key, e.hostKey = newPrivateKey(t)
	for name, b := range map[string][]byte{
		"ssh_private_key": e.key,
		"access_key":      []byte("AKIDEXAMPLE\n"),
		"secret_key":      []byte(s3Secret),
		"token":           []byte(httpToken + "\n"),
	} {
		e.addKey(t, name, b)
	}
	return e
}

func (e credsEnv) addKey(t *testing.T, name string, b []byte) {
	t.Helper()
	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	if sec.HasKey(name) {
		require.NoError(t, sec.RemoveKey(name))
	}
	require.NoError(t, sec.AddKey(name, b))
}

// store returns a store of the url with the keywords, one per line.
func (e credsEnv) store(t *testing.T, url string, keywords ...string) *T {
	t.Helper()
	return newStore(t, "[store#lab]\ntype = plakar\nurl = "+url+"\npassphrase = from system/sec/backup key passphrase\n"+strings.Join(keywords, "\n")+"\n")
}

func (e credsEnv) sftpStore(t *testing.T) *T {
	t.Helper()
	return e.store(t, "sftp://backup@h/data", "ssh_private_key = from system/sec/backup key ssh_private_key", "host_key = "+e.hostKey)
}

// entry returns the entry of the store in the stores.yml of the directory.
func entry(t *testing.T, d dirs) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(d.config, "stores.yml"))
	require.NoError(t, err)
	var cfg struct {
		Stores map[string]map[string]string `yaml:"stores"`
	}
	require.NoError(t, yaml.Unmarshal(b, &cfg))
	return cfg.Stores["store"]
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

// identities returns the identity files of the configuration directory.
func identities(t *testing.T, d dirs) []string {
	t.Helper()
	l, err := filepath.Glob(filepath.Join(d.config, identityPrefix+"*"))
	require.NoError(t, err)
	return l
}

// TestSFTPStoreLogsInWithItsKeyOnly pins what a sftp:// store hands plakar:
// the key and the known hosts in files of its private directory only their
// owner reads, named by the options of its entry, and an ssh command, first
// in the PATH of plakar, keeping ssh off the configuration, the agent, the
// identities and the known hosts of the user om runs as.
func TestSFTPStoreLogsInWithItsKeyOnly(t *testing.T) {
	e := newCredsEnv(t)
	t.Setenv("PATH", "/usr/bin:/bin")
	s := e.sftpStore(t)
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))

	ids := identities(t, e.dirs)
	require.Len(t, ids, 1)
	knownHosts := filepath.Join(e.dirs.config, knownHostsName)
	b, err := os.ReadFile(ids[0])
	require.NoError(t, err)
	assert.Equal(t, string(e.key), string(b))
	b, err = os.ReadFile(knownHosts)
	require.NoError(t, err)
	assert.Equal(t, "* "+e.hostKey+"\n", string(b), "the key is the one of the server whatever its name")
	for _, path := range []string{ids[0], knownHosts, filepath.Join(e.dirs.config, "stores.yml")} {
		assert.Equal(t, os.FileMode(0600), mode(t, path), path)
	}
	assert.Equal(t, os.FileMode(0700), mode(t, e.dirs.config))

	got := entry(t, e.dirs)
	assert.Equal(t, "sftp://backup@h/data", got["location"])
	assert.Equal(t, ids[0], got["identity"])
	assert.Equal(t, "* "+e.hostKey, got["host_key"])
	assert.NotContains(t, got, "ssh_private_key", "the key is not handed to the agent of the user")

	wrapper := filepath.Join(e.dirs.bin, wrapperName)
	assert.Equal(t, os.FileMode(0700), mode(t, wrapper))
	assert.Equal(t, os.FileMode(0700), mode(t, e.dirs.bin))
	b, err = os.ReadFile(wrapper)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\n# Written by om for the store lab: do not edit.\n"+
		"exec /usr/bin/ssh -F /dev/null -o IdentitiesOnly=yes -o IdentityAgent=none -o IdentityFile="+ids[0]+
		" -o CertificateFile=none -o PreferredAuthentications=publickey -o UserKnownHostsFile="+knownHosts+
		" -o GlobalKnownHostsFile=/dev/null -o StrictHostKeyChecking=yes -o UpdateHostKeys=no \"$@\"\n", string(b))

	env, err := os.ReadFile(filepath.Join(fakeDir, "env.1"))
	require.NoError(t, err)
	assert.Contains(t, "\n"+string(env), "\nPATH="+e.dirs.bin+":/usr/bin:/bin\n", "the ssh of the store is the first of the PATH of plakar")

	// No PATH: never an empty element, which is the working directory.
	t.Setenv("PATH", "")
	assert.Equal(t, []string{"PATH=" + e.dirs.bin + ":" + defaultPath}, s.env(e.dirs))
}

// TestSFTPStoreRunsItsSSHCommand pins the command the sftp package finds in
// the PATH run with the options of the store before the ones of the
// package, which ssh then ignores, and from a directory commands run from:
// the run directory, a memory filesystem, may be mounted noexec.
func TestSFTPStoreRunsItsSSHCommand(t *testing.T) {
	e := newCredsEnv(t)
	fake := filepath.Join(t.TempDir(), "ssh")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0755))
	sshPath = func() (string, error) { return fake, nil }
	s := e.sftpStore(t)
	_, err := s.prepare()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(e.dirs.bin, filepath.Join(e.root, "var")), e.dirs.bin)
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-i", "/root/.ssh/id_rsa", "--", "h")
	cmd.Env = s.env(e.dirs)
	// exec.Command resolved ssh in the PATH of the test: resolve it in
	// the one of the store, as the package does.
	cmd.Path = filepath.Join(strings.TrimPrefix(strings.SplitN(cmd.Env[0], ":", 2)[0], "PATH="), "ssh")
	out, err := cmd.Output()
	require.NoError(t, err)
	args := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.Equal(t, []string{"-F", "/dev/null", "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none"}, args[:6])
	assert.Equal(t, []string{"-o", "BatchMode=yes", "-i", "/root/.ssh/id_rsa", "--", "h"}, args[len(args)-6:])
}

// TestCredentialsNeverReachACommandLineTheEnvironmentNorTheLogs pins the
// secrets of every scheme out of the argv and the environment of plakar,
// and out of the logs at their most verbose.
func TestCredentialsNeverReachACommandLineTheEnvironmentNorTheLogs(t *testing.T) {
	e := newCredsEnv(t)
	e.createStore(t)
	keyBody := strings.Split(strings.TrimSpace(string(e.key)), "\n")[1]
	stores := map[string]*T{
		"sftp":  e.sftpStore(t),
		"s3":    e.store(t, "s3://h/bucket", "access_key = from system/sec/backup key access_key", "secret_key = from system/sec/backup key secret_key"),
		"https": e.store(t, "https://h:9876/", "token = from system/sec/backup key token"),
	}
	for name, s := range stores {
		resetFake(t)
		var logs strings.Builder
		s.SetLoggerForTest(plog.NewLogger(zerolog.New(&logs).Level(zerolog.TraceLevel)))
		require.NoError(t, s.Check(context.Background(), backup.RunFilter{}), name)
		require.NoError(t, s.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil), name)
		seen := logs.String()
		for _, file := range []string{"calls", "env.1", "env.2"} {
			b, err := os.ReadFile(filepath.Join(fakeDir, file))
			require.NoError(t, err, name)
			seen += string(b)
		}
		for _, secret := range []string{passphrase, keyBody, s3Secret, httpToken} {
			assert.NotContains(t, seen, secret, name)
		}
	}
}

// TestS3StoreSignsWithItsKeysOnly pins the keys of a s3:// store in the
// options of its entry, a trailing newline dropped, the static credentials
// the package then never looks past.
func TestS3StoreSignsWithItsKeysOnly(t *testing.T) {
	e := newCredsEnv(t)
	s := e.store(t, "s3://h/bucket", "access_key = from system/sec/backup key access_key", "secret_key = from system/sec/backup key secret_key")
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	got := entry(t, e.dirs)
	assert.Equal(t, "AKIDEXAMPLE", got["access_key"])
	assert.Equal(t, s3Secret, got["secret_access_key"])
	assert.Equal(t, os.FileMode(0600), mode(t, filepath.Join(e.dirs.config, "stores.yml")))
	assert.NoFileExists(t, filepath.Join(e.dirs.bin, wrapperName))
}

// TestHTTPSStoreSendsItsTokenAndTrustsItsCA pins the token in the option of
// the entry, and the certificate authorities of ca alone trusted, through
// the files the tls library of plakar reads its roots from.
func TestHTTPSStoreSendsItsTokenAndTrustsItsCA(t *testing.T) {
	e := newCredsEnv(t)
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	e.addKey(t, "ca", ca)

	s := e.store(t, "https://h:9876/", "token = from system/sec/backup key token")
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	assert.Equal(t, httpToken, entry(t, e.dirs)["auth_token"])
	env, err := os.ReadFile(filepath.Join(fakeDir, "env.1"))
	require.NoError(t, err)
	assert.NotContains(t, string(env), "SSL_CERT_", "no ca: the certificate authorities of the system")

	resetFake(t)
	s = e.store(t, "https://h:9876/", "ca = from system/sec/backup key ca")
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	assert.NotContains(t, entry(t, e.dirs), "auth_token", "the token unset is no longer in the entry")
	caFile := filepath.Join(e.dirs.config, caName)
	b, err := os.ReadFile(caFile)
	require.NoError(t, err)
	assert.Equal(t, string(ca), string(b))
	assert.Equal(t, os.FileMode(0600), mode(t, caFile))
	entries, err := os.ReadDir(filepath.Join(e.dirs.config, caDirName))
	require.NoError(t, err)
	assert.Empty(t, entries)
	env, err = os.ReadFile(filepath.Join(fakeDir, "env.1"))
	require.NoError(t, err)
	assert.Contains(t, string(env), "SSL_CERT_FILE="+caFile+"\n")
	assert.Contains(t, string(env), "SSL_CERT_DIR="+filepath.Join(e.dirs.config, caDirName)+"\n")

	e.addKey(t, "notca", []byte("not a certificate"))
	err = e.store(t, "https://h:9876/", "ca = from system/sec/backup key notca").Check(context.Background(), backup.RunFilter{})
	assert.ErrorContains(t, err, "ca: no PEM certificate found")
}

// TestRemoteStoreWithoutItsCredentialsIsRefusedBeforeAnyCall pins the
// refusal of a location plakar would reach with what the node holds, the
// ssh setup of root or the role of the instance, of a credential keyword of
// another scheme, and of a credential plakar cannot read, with no plakar
// call, no file written and no secret in the error.
func TestRemoteStoreWithoutItsCredentialsIsRefusedBeforeAnyCall(t *testing.T) {
	e := newCredsEnv(t)
	e.addKey(t, "notakey", []byte("not a private key"))
	e.addKey(t, "lines", []byte("s3-secret\nnot-to-be-seen"))
	sshKey := "ssh_private_key = from system/sec/backup key ssh_private_key"
	hostKey := "host_key = " + e.hostKey
	accessKey := "access_key = from system/sec/backup key access_key"
	secretKey := "secret_key = from system/sec/backup key secret_key"
	token := "token = from system/sec/backup key token"
	cases := []struct {
		url      string
		keywords []string
		err      string
	}{
		{"sftp://backup@h/data", nil, "a sftp:// store needs ssh_private_key and host_key"},
		{"sftp://backup@h/data", []string{sshKey}, "a sftp:// store needs ssh_private_key and host_key"},
		{"sftp://backup@h/data", []string{hostKey}, "a sftp:// store needs ssh_private_key and host_key"},
		{"sftp://backup@h/data", []string{"ssh_private_key = from system/sec/backup key nosuch", hostKey}, "ssh_private_key: "},
		{"sftp://backup@h/data", []string{"ssh_private_key = from system/sec/backup key notakey", hostKey}, "ssh_private_key: "},
		{"sftp://backup@h/data", []string{"ssh_private_key = -----BEGIN OPENSSH PRIVATE KEY-----", hostKey}, "ssh_private_key: "},
		{"sftp://backup@h/data", []string{sshKey, "host_key = not a key"}, "host_key: "},
		{"sftp://backup@h/data", []string{sshKey, hostKey, token}, "token is a keyword of a https:// store"},
		{"s3://h/bucket", nil, "a s3:// store needs access_key and secret_key"},
		{"s3://h/bucket", []string{accessKey}, "a s3:// store needs access_key and secret_key"},
		{"s3://h/bucket", []string{secretKey}, "a s3:// store needs access_key and secret_key"},
		{"s3://h/bucket", []string{accessKey, "secret_key = from system/sec/backup key lines"}, "secret_key: the key holds a line break"},
		{"s3://h/bucket", []string{accessKey, "secret_key = " + s3Secret}, "secret_key: "},
		{"s3://h/bucket", []string{accessKey, secretKey, sshKey}, "ssh_private_key is a keyword of a sftp:// store"},
		{"http://h:9876/", []string{token}, "token is a keyword of a https:// store"},
		{"https://h:9876/", []string{hostKey}, "host_key is a keyword of a sftp:// store"},
		{e.storePath(), []string{accessKey}, "access_key is a keyword of a s3:// store"},
	}
	for _, tc := range cases {
		name := fmt.Sprintf("%s %v", tc.url, tc.keywords)
		s := e.store(t, tc.url, tc.keywords...)
		for _, err := range []error{
			s.Check(context.Background(), backup.RunFilter{}),
			s.Backup(context.Background(), run1, "fs#1", e.src, e.src, nil),
			s.Complete(context.Background(), run1),
			s.Delete(context.Background(), []backup.Run{run1}),
		} {
			if assert.Error(t, err, name) {
				assert.Contains(t, err.Error(), tc.err, name)
				assert.NotContains(t, err.Error(), "not-to-be-seen", name)
			}
		}
		_, err := s.Runs(context.Background(), backup.RunFilter{})
		assert.Error(t, err, name)
	}
	assert.Empty(t, calls(t))
	assert.Empty(t, identities(t, e.dirs))
	assert.NoFileExists(t, filepath.Join(e.dirs.config, "stores.yml"))
	assert.NoFileExists(t, filepath.Join(e.dirs.bin, wrapperName))

	// A plakar server needs no credential, and a store of the node none.
	require.NoError(t, e.store(t, "https://h:9876/").Check(context.Background(), backup.RunFilter{}))
	require.NoError(t, e.store(t, "http://h:9876/").Check(context.Background(), backup.RunFilter{}))
	e.createStore(t)
	require.NoError(t, e.store(t, e.storePath()).Check(context.Background(), backup.RunFilter{}))
}

// TestCredentialFilesFollowTheKeywords pins the files of the private
// directory written again when stale, the key changed being a file of
// another name, so a connection of its own, left alone when current, and
// removed with the keyword: the key of a store does not outlive it on the
// node.
func TestCredentialFilesFollowTheKeywords(t *testing.T) {
	e := newCredsEnv(t)
	s := e.sftpStore(t)
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	ids := identities(t, e.dirs)
	require.Len(t, ids, 1)
	knownHosts := filepath.Join(e.dirs.config, knownHostsName)
	wrapper := filepath.Join(e.dirs.bin, wrapperName)
	before := inode(t, ids[0])

	// Current: left alone.
	require.NoError(t, e.sftpStore(t).Check(context.Background(), backup.RunFilter{}))
	assert.Equal(t, before, inode(t, ids[0]))

	// Readable by others, or changed on the node: written again.
	require.NoError(t, os.Chmod(ids[0], 0644))
	require.NoError(t, os.WriteFile(knownHosts, []byte("* ssh-ed25519 AAAA\n"), 0600))
	require.NoError(t, os.WriteFile(wrapper, []byte("#!/bin/sh\nexec /usr/bin/ssh \"$@\"\n"), 0700))
	require.NoError(t, e.sftpStore(t).Check(context.Background(), backup.RunFilter{}))
	assert.Equal(t, os.FileMode(0600), mode(t, ids[0]))
	b, err := os.ReadFile(knownHosts)
	require.NoError(t, err)
	assert.Equal(t, "* "+e.hostKey+"\n", string(b))
	b, err = os.ReadFile(wrapper)
	require.NoError(t, err)
	assert.Contains(t, string(b), "IdentitiesOnly=yes")

	// The key changed: a file of another name, the former removed.
	key, _ := newPrivateKey(t)
	e.addKey(t, "ssh_private_key", key)
	require.NoError(t, e.sftpStore(t).Check(context.Background(), backup.RunFilter{}))
	changed := identities(t, e.dirs)
	require.Len(t, changed, 1)
	assert.NotEqual(t, ids[0], changed[0])
	assert.Equal(t, changed[0], entry(t, e.dirs)["identity"])

	// The host key changed: another name too.
	_, hostKey := newPrivateKey(t)
	e.hostKey = hostKey
	require.NoError(t, e.sftpStore(t).Check(context.Background(), backup.RunFilter{}))
	rehosted := identities(t, e.dirs)
	require.Len(t, rehosted, 1)
	assert.NotEqual(t, changed[0], rehosted[0])

	// The store moved to a location with no key: nothing is left.
	require.NoError(t, e.store(t, "https://h:9876/").Check(context.Background(), backup.RunFilter{}))
	assert.Empty(t, identities(t, e.dirs))
	assert.NoFileExists(t, knownHosts)
	assert.NoFileExists(t, wrapper)
	got := entry(t, e.dirs)
	assert.NotContains(t, got, "identity")
	assert.NotContains(t, got, "host_key")
}

// TestHostKeyIsAValueOrAKey pins the host key written in the section, with
// or without the host before it, or read from a key.
func TestHostKeyIsAValueOrAKey(t *testing.T) {
	e := newCredsEnv(t)
	e.addKey(t, "host_key", []byte("[h]:2222 "+e.hostKey+" a comment\n"))
	sshKey := "ssh_private_key = from system/sec/backup key ssh_private_key"
	for _, hostKey := range []string{e.hostKey, "h " + e.hostKey, "from system/sec/backup key host_key"} {
		s := e.store(t, "sftp://backup@h:2222/data", sshKey, "host_key = "+hostKey)
		require.NoError(t, s.Check(context.Background(), backup.RunFilter{}), hostKey)
		assert.Equal(t, "* "+e.hostKey, entry(t, e.dirs)["host_key"], hostKey)
	}
}

// TestSecStoreReadsItsCredentialsAsItsReader pins the credentials of a
// store declared in a sec read from that sec, written from ., as the object
// using it, into directories of its own.
func TestSecStoreReadsItsCredentialsAsItsReader(t *testing.T) {
	e := newCredsEnv(t)
	p, err := naming.ParsePath("test/sec/stores")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	require.NoError(t, sec.AddKey("passphrase", []byte(passphrase)))
	require.NoError(t, sec.AddKey("ssh_private_key", e.key))
	s := newStore(t, "[store#lab]\ntype = plakar\nurl = sftp://backup@h/data\npassphrase = from . key passphrase\nssh_private_key = from . key ssh_private_key\nhost_key = "+e.hostKey+"\n")
	s.SetOwner(p)
	s.SetReader("test")
	require.NoError(t, s.Check(context.Background(), backup.RunFilter{}))
	d, err := s.dirs()
	require.NoError(t, err)
	assert.Len(t, identities(t, d), 1)
	assert.Empty(t, identities(t, e.dirs), "nothing in the directories of the store of the node of the same name")
	assert.FileExists(t, filepath.Join(d.bin, wrapperName))

	// An object of another namespace, the sec not shared with it, reads
	// no key.
	other := newStore(t, "[store#lab]\ntype = plakar\nurl = sftp://backup@h/data\npassphrase = from . key passphrase\nssh_private_key = from . key ssh_private_key\nhost_key = "+e.hostKey+"\n")
	other.SetOwner(p)
	other.SetReader("elsewhere")
	require.Error(t, other.Check(context.Background(), backup.RunFilter{}))
}

// TestWithTheRealPlakarOverHTTPS pins what the fake cannot of a https://
// store: the plakar of the node sends the token of the entry, verifies its
// server with the certificate authorities of ca, and refuses the server
// with the ones of the system, or of another ca. It is skipped on a host
// without plakar.
func TestWithTheRealPlakarOverHTTPS(t *testing.T) {
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	e := newCredsEnv(t)
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)

	var mu sync.Mutex
	var authorizations []string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations = append(authorizations, r.Header.Get("Authorization"))
		mu.Unlock()
		http.NotFound(w, r)
	})
	seen := func() []string {
		mu.Lock()
		defer mu.Unlock()
		l := authorizations
		authorizations = nil
		return l
	}
	srv := httptest.NewUnstartedServer(handler)
	// The handshakes plakar gives up are not logged on the test output.
	srv.Config.ErrorLog = nil
	srv.TLS = &tls.Config{}
	srv.StartTLS()
	defer srv.Close()
	e.addKey(t, "ca", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}))
	otherSrv := httptest.NewTLSServer(handler)
	defer otherSrv.Close()

	token := "token = from system/sec/backup key token"
	ca := "ca = from system/sec/backup key ca"

	// The server is not a plakar server: the call fails once its answer is
	// read, the token sent.
	err = e.store(t, srv.URL+"/", token, ca).Check(context.Background(), backup.RunFilter{})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "x509")
	l := seen()
	require.NotEmpty(t, l, "the server of the ca is talked to")
	assert.Equal(t, "Bearer "+httpToken, l[0])

	// Without ca, the certificate authorities of the system do not know
	// the server.
	err = e.store(t, srv.URL+"/", token).Check(context.Background(), backup.RunFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x509")
	assert.Empty(t, seen(), "no request, so no token, reaches a server not verified")
}
