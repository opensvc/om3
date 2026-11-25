package storeplakar

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/executable"
	"github.com/opensvc/om3/v3/util/plog"
)

// TestWithTheRealPlakar runs the driver against the plakar binary of the
// host, in the directories of the test, to pin what the fake cannot: the
// tags plakar selects on, the store created by the first write only, the
// space reclaimed after a deletion and the maintenance giving up under the
// lock of a backup. It is skipped on a host without plakar.
func TestWithTheRealPlakar(t *testing.T) {
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	e := newTestEnv(t)
	disableSecurityCheck = runDisableSecurityCheck
	warmUp = runWarmUp
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	// plakar reads the passphrase from om, which the test does not build.
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	var logs bytes.Buffer
	e.store.SetLoggerForTest(plog.NewLogger(zerolog.New(&logs).Level(zerolog.InfoLevel)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	require.NoError(t, os.WriteFile(filepath.Join(e.src, "a"), []byte("content of a\n"), 0644))
	run := run1
	run.ClusterID = "5a0ae2ee-2f5b-4a0c-9d9a-0f4b3a3f4c11"
	other := run1
	other.ID = "r2"
	other.ObjectID = "o2"
	other.Path = "test/svc/app2"
	other.ClusterID = "9e1f2a30-7c44-4c57-8a0b-6d2c5e1b7f22"
	other.ClusterName = "cluster2"

	// No read creates the store, and a store under the head is refused
	// before it is created.
	runs, err := e.store.Runs(ctx, backup.RunFilter{Incomplete: true})
	require.NoError(t, err)
	assert.Empty(t, runs)
	require.Error(t, e.store.Check(ctx, backup.RunFilter{}))
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run}))
	require.Error(t, e.store.Backup(ctx, run, "fs#1", e.root, e.root, nil))
	require.NoDirExists(t, e.storePath())

	// The first write creates it.
	require.NoError(t, e.store.Backup(ctx, run, "fs#1", e.src, e.src, nil))
	require.FileExists(t, filepath.Join(e.storePath(), "CONFIG"))
	// The plakar of the store no longer calls out for security updates.
	require.FileExists(t, filepath.Join(e.dirs.cache, securityCheckMarker))
	cookies, err := filepath.Glob(filepath.Join(e.dirs.cache, "cookies", "*", ".disabled-security-check"))
	require.NoError(t, err)
	require.NotEmpty(t, cookies, "plakar records the security check disabled in its cache")
	require.NoError(t, e.store.BackupStream(ctx, run, backup.MemberConfig, strings.NewReader("[DEFAULT]\nid = o1\n")))
	require.NoError(t, e.store.Complete(ctx, run))
	require.NoError(t, e.store.Backup(ctx, other, "fs#1", e.src, e.src, nil))
	require.NoError(t, e.store.Complete(ctx, other))

	// The listing narrowed with the tags of the cluster and of the
	// object holds the runs of that object only: plakar requires both.
	snaps, err := e.store.list(ctx, tagValue{tagCluster, run.ClusterID}, tagValue{tagObject, run.ObjectID})
	require.NoError(t, err)
	require.Len(t, snaps, 3, "the two members and the marker")
	for _, s := range snaps {
		assert.Equal(t, run.ID, s.Tags[tagRun])
	}
	snaps, err = e.store.list(ctx, tagValue{tagCluster, run.ClusterID}, tagValue{tagObject, other.ObjectID})
	require.NoError(t, err)
	assert.Empty(t, snaps, "the tags of a -tag are all required")
	require.NoError(t, e.store.Close())

	found, err := backup.FindRun(ctx, e.store, backup.RunFilter{ObjectID: run.ObjectID, Cluster: run.ClusterID})
	require.NoError(t, err)
	assert.Equal(t, run.ID, found.ID)
	assert.False(t, found.CompletedAt.IsZero())
	assert.Equal(t, []string{"fs#1", backup.MemberConfig}, found.MemberNames())
	runs, err = e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 2)
	stats, err := e.store.ObjectStats(ctx)
	require.NoError(t, err)
	assert.Len(t, stats, 2)

	// The members are read back from the snapshots the listing names.
	require.NoError(t, e.store.Restore(ctx, found, "fs#1", e.dst))
	b, err := os.ReadFile(filepath.Join(e.dst, "a"))
	require.NoError(t, err)
	assert.Equal(t, "content of a\n", string(b))
	b, err = backup.ReadMember(ctx, e.store, found, backup.MemberConfig)
	require.NoError(t, err)
	assert.Equal(t, "[DEFAULT]\nid = o1\n", string(b))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{ID: run.ID}))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{}))

	// A deletion removes the run and runs the maintenance.
	logs.Reset()
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run}))
	assert.Contains(t, logs.String(), "at @store maintenance")
	assert.NotContains(t, logs.String(), "maintenance skipped")
	runs, err = e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, other.ID, runs[0].ID)

	// A deletion removing nothing runs none.
	logs.Reset()
	require.NoError(t, e.store.Delete(ctx, []backup.Run{run}))
	assert.NotContains(t, logs.String(), "at @store maintenance")

	// Under the lock of a backup, here one whose stream stays open, the
	// maintenance gives up and the driver says it skipped it.
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	writer := e.store
	third := run
	third.ID = "r3"
	go func() {
		done <- writer.BackupStream(ctx, third, backup.MemberConfig, pr)
	}()
	logs.Reset()
	skipped := false
	for i := 0; i < 20 && !skipped; i++ {
		time.Sleep(500 * time.Millisecond)
		require.NoError(t, e.store.maintain(ctx))
		skipped = strings.Contains(logs.String(), "maintenance skipped: repository busy")
	}
	_, err = pw.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, pw.Close())
	require.NoError(t, <-done)
	assert.True(t, skipped, "the maintenance under the lock of a backup is skipped, not failed")
}

// The environment of TestWithTheRealPlakarOverSFTP: the user and the host of
// a sftp server, as backup@[fd01::1], a directory of the server the user
// writes in, the file of a private key the server accepts for the user, and
// the public key of the server.
const (
	sftpHostEnv    = "OM_TEST_PLAKAR_SFTP_HOST"
	sftpDirEnv     = "OM_TEST_PLAKAR_SFTP_DIR"
	sftpKeyEnv     = "OM_TEST_PLAKAR_SFTP_KEY"
	sftpHostKeyEnv = "OM_TEST_PLAKAR_SFTP_HOST_KEY"
)

// newPrivateKey returns a new ssh private key, which no server accepts, and
// its public key as ssh-keyscan prints one.
func newPrivateKey(t *testing.T) ([]byte, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return pem.EncodeToMemory(block), strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
}

// TestWithTheRealPlakarOnAPlainHTTPS3Endpoint runs the s3 plugin an
// operator installed against a server of the test answering in plain http,
// and pins what the fake cannot: the message plakar fails its TLS handshake
// with is one the driver knows, so its error carries the hint, and the
// use_tls the driver writes for disable_tls is one the plugin reads, the
// call then reaching the server, which holds no bucket. It needs plakar
// with its s3 plugin.
func TestWithTheRealPlakarOnAPlainHTTPS3Endpoint(t *testing.T) {
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	if l, _ := filepath.Glob(filepath.Join(defaultDataDir(), "plugins", "*", "s3_*")); len(l) == 0 {
		t.Skip("no s3 plugin of plakar in " + defaultDataDir())
	}
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	newTestEnv(t)
	dataDir = defaultDataDir
	disableSecurityCheck = runDisableSecurityCheck
	warmUp = runWarmUp
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	require.NoError(t, sec.AddKey("access_key", []byte("AKIDEXAMPLE")))
	require.NoError(t, sec.AddKey("secret_key", []byte("s3-secret")))
	store := func(keywords string) *T {
		return newStore(t, "[store#lab]\ntype = plakar\nurl = s3://"+strings.TrimPrefix(srv.URL, "http://")+"/bucket\npassphrase = from system/sec/backup key passphrase\naccess_key = from system/sec/backup key access_key\nsecret_key = from system/sec/backup key secret_key\n"+keywords)
	}

	// With TLS, the default, the handshake fails on the http response of
	// the server, which gets no request, and the error says what to set.
	err = store("").Check(ctx, backup.RunFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server gave HTTP response to HTTPS client")
	assert.Contains(t, err.Error(), plainHTTPHint)
	assert.Zero(t, requests.Load(), "no request is sent in plain http by itself")

	// Without TLS, the request reaches the server, which has no bucket.
	err = store("disable_tls = true\n").Check(ctx, backup.RunFilter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bucket does not exist")
	assert.NotContains(t, err.Error(), "disable_tls")
	assert.NotZero(t, requests.Load(), "the request is sent in plain http")
}

// TestWithTheRealPlakarOverSFTP runs a backup and a restore through the sftp
// plugin an operator installed, in the data directory plakar installs the
// plugins in, with the key of a sec and the host key of the section, and
// pins what the fake cannot: the ssh of the plugin logs in with the key of
// the store only, and to the server of its host key only, and the store is
// created by its first writes only, once for the ones made at once.
//
// The test runs an ssh agent holding the key the server accepts, as the
// user running om may: a store whose key the server refuses must fail, the
// agent not being asked. It needs plakar with its sftp plugin and the
// variables above, and removes the store it creates.
func TestWithTheRealPlakarOverSFTP(t *testing.T) {
	host := os.Getenv(sftpHostEnv)
	if host == "" {
		t.Skip("no " + sftpHostEnv)
	}
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	keyFile, hostKey, base := os.Getenv(sftpKeyEnv), os.Getenv(sftpHostKeyEnv), os.Getenv(sftpDirEnv)
	require.NotEmpty(t, keyFile, sftpKeyEnv)
	require.NotEmpty(t, hostKey, sftpHostKeyEnv)
	require.NotEmpty(t, base, sftpDirEnv)
	key, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	u, err := url.Parse("sftp://" + host)
	require.NoError(t, err)

	e := newTestEnv(t)
	dataDir = defaultDataDir
	disableSecurityCheck = runDisableSecurityCheck
	warmUp = runWarmUp
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	sshPath = func() (string, error) { return exec.LookPath("ssh") }
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// The plugin keeps its ssh connections in the runtime directory of the
	// user: a short one of the test, a socket path being bounded, whose
	// connections are closed at the end.
	runtimeDir, err := os.MkdirTemp("/tmp", "pk")
	require.NoError(t, err)
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Cleanup(func() {
		socks, _ := filepath.Glob(filepath.Join(runtimeDir, "plakar", "ssh", "*.sock"))
		for _, sock := range socks {
			_ = exec.Command("ssh", "-S", sock, "-O", "exit", "--", u.Hostname()).Run()
		}
		_ = os.RemoveAll(runtimeDir)
	})

	// The agent of the user holds a key the server accepts, which a plain
	// ssh logs in with.
	agentSock := filepath.Join(runtimeDir, "agent")
	out, err := exec.Command("ssh-agent", "-a", agentSock).Output()
	require.NoError(t, err)
	for _, line := range strings.Split(string(out), "\n") {
		if pid, ok := strings.CutPrefix(line, "SSH_AGENT_PID="); ok {
			pid, _, _ = strings.Cut(pid, ";")
			t.Cleanup(func() { _ = exec.Command("kill", pid).Run() })
		}
	}
	t.Setenv("SSH_AUTH_SOCK", agentSock)
	addKey := filepath.Join(runtimeDir, "key")
	require.NoError(t, os.WriteFile(addKey, key, 0600))
	b, err := exec.Command("ssh-add", addKey).CombinedOutput()
	require.NoError(t, err, string(b))
	require.NoError(t, os.Remove(addKey))
	knownHosts := filepath.Join(runtimeDir, "known_hosts")
	line, err := knownHostsLine([]byte(hostKey))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(knownHosts, []byte(line+"\n"), 0600))
	b, err = exec.Command("ssh", "-F", "/dev/null", "-o", "BatchMode=yes", "-o", "UserKnownHostsFile="+knownHosts,
		"-o", "GlobalKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=yes", "-l", u.User.Username(), "--", u.Hostname(), "-s", "sftp").CombinedOutput()
	if err != nil && !bytes.Contains(b, []byte("Permission denied")) {
		// A server serving sftp only closes a session with no input.
		err = nil
	}
	require.NoError(t, err, "the key of the agent is one the server accepts: %s", string(b))

	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	refused, otherHostKey := newPrivateKey(t)
	require.NoError(t, sec.AddKey("ssh_private_key", key))
	require.NoError(t, sec.AddKey("refused", refused))

	dir := path.Join(base, "plakar-test-"+uuid.NewString()[:8])
	// The stores log their plakar calls, some at once, the ones reading
	// the store included, which are logged as traces.
	level := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(level) })
	var logs bytes.Buffer
	log := plog.NewLogger(zerolog.New(zerolog.SyncWriter(&logs)).Level(zerolog.TraceLevel))
	named := func(name, dir, sshKey, hostKey string) *T {
		config := "[store#" + name + "]\ntype = plakar\nurl = sftp://" + host + dir + "\npassphrase = from system/sec/backup key passphrase\n"
		if sshKey != "" {
			config += "ssh_private_key = from system/sec/backup key " + sshKey + "\n"
		}
		if hostKey != "" {
			config += "host_key = " + hostKey + "\n"
		}
		cfg, err := xconfig.NewObject("", []byte(config))
		require.NoError(t, err)
		drv := backup.GetDriver("plakar")
		drv.SetName(name)
		drv.SetConfig(cfg)
		s := drv.(*T)
		s.SetLoggerForTest(log)
		return s
	}
	store := func(sshKey, hostKey string) *T {
		return named("lab", dir, sshKey, hostKey)
	}
	create := func(s *T) error {
		// A create by hand, as the operator may still do.
		if _, err := s.prepare(); err != nil {
			return err
		}
		_, err := s.run(ctx, zerolog.InfoLevel, "create")
		return err
	}
	t.Cleanup(func() { removeRemote(t, u, key, hostKey, dir) })

	// A store with no key is refused before any plakar call: the agent is
	// not what logs it in.
	err = create(store("", hostKey))
	require.ErrorContains(t, err, "needs ssh_private_key and host_key")
	err = create(store("ssh_private_key", ""))
	require.ErrorContains(t, err, "needs ssh_private_key and host_key")

	// A store whose key the server refuses fails, the agent holding the
	// key it accepts.
	err = create(store("refused", hostKey))
	require.Error(t, err, "the key of the agent must not log the store in")
	assert.Contains(t, err.Error(), "Permission denied")

	// A server presenting another key than host_key is refused.
	err = create(store("ssh_private_key", otherHostKey))
	require.Error(t, err, "the server is not the one of host_key")
	assert.Contains(t, err.Error(), "Host key verification failed")

	// None of the stores refused created anything.
	removeRemote(t, u, key, hostKey, dir)

	e.store = store("ssh_private_key", hostKey)
	assert.False(t, e.store.IsLocal())
	require.NoError(t, os.WriteFile(filepath.Join(e.src, "a"), []byte("content of a\n"), 0644))

	// No read creates the store, nor the marker of a run: each fails on
	// the answer of the sftp package that the location holds no file.
	logs.Reset()
	_, err = e.store.Runs(ctx, backup.RunFilter{})
	require.ErrorIs(t, err, errNoRepository, "the sftp backend of the plugins installed is found, and the key of the sec logs in")
	require.ErrorIs(t, e.store.Check(ctx, backup.RunFilter{}), errNoRepository)
	require.ErrorIs(t, e.store.Delete(ctx, []backup.Run{run1}), errNoRepository)
	require.ErrorIs(t, e.store.Complete(ctx, run1), errNoRepository)
	assert.NotContains(t, logs.String(), "at @store create")

	// A location the user of the store cannot write in is not one holding
	// no repository, and one it cannot create in fails the write.
	denied := named("lab", "/plakar-test-"+uuid.NewString()[:8]+"/x", "ssh_private_key", hostKey)
	err = denied.Backup(ctx, run1, "fs#1", e.src, e.src, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plakar create: ")
	assert.Contains(t, err.Error(), "permission denied", logs.String())

	// The first writes create the store, once: the ones of two objects of
	// the node, sharing the private directories of the store and so the
	// lock of its creation, and the one of another node, here a store of
	// another name with its own directories, whose create finds the
	// location taken or the store created.
	second, third := run1, run1
	second.ID, second.ObjectID, second.Path = "r2", "o2", "test/svc/app2"
	third.ID, third.ObjectID, third.Path = "r3", "o3", "test/svc/app3"
	writers := []*T{e.store, store("ssh_private_key", hostKey), named("lab2", dir, "ssh_private_key", hostKey)}
	logs.Reset()
	t0 := time.Now()
	errs := make(chan error, len(writers))
	for i, run := range []backup.Run{run1, second, third} {
		go func() {
			if i == 1 {
				// A stream, which looks for the store before it is read.
				errs <- writers[i].BackupStream(ctx, run, backup.MemberConfig, strings.NewReader("[DEFAULT]\nid = o2\n"))
				return
			}
			errs <- writers[i].Backup(ctx, run, "fs#1", e.src, e.src, nil)
		}()
	}
	for range writers {
		require.NoError(t, <-errs)
	}
	t.Logf("3 first writes at once, the store created: %s", time.Since(t0).Round(10*time.Millisecond))
	creates := plakarCalls(logs.String(), bin, "create")
	t.Logf("plakar calls of the 3 first writes: %d, %d of them a create", plakarCalls(logs.String(), bin, ""), creates)
	assert.GreaterOrEqual(t, creates, 1)
	assert.LessOrEqual(t, creates, 2, "one create for the two writers of the node, one at most for the other node")
	ids := make(map[string]bool)
	for _, s := range writers {
		id, err := s.repositoryID(ctx)
		require.NoError(t, err)
		ids[id] = true
	}
	require.Len(t, ids, 1, "one repository, the same for every writer")

	// The store is there: a write is one plakar call.
	logs.Reset()
	t0 = time.Now()
	require.NoError(t, e.store.Backup(ctx, run1, "fs#2", e.src, e.src, nil))
	t.Logf("a write into the store created: %s", time.Since(t0).Round(10*time.Millisecond))
	assert.Equal(t, 1, plakarCalls(logs.String(), bin, ""), logs.String())

	require.NoError(t, e.store.Complete(ctx, run1))
	require.NoError(t, writers[1].Complete(ctx, second))
	require.NoError(t, writers[2].Complete(ctx, third))
	runs, err := e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 3, "the runs of every writer are read with the key of the one repository")
	found, err := backup.FindRun(ctx, e.store, backup.RunFilter{ID: second.ID})
	require.NoError(t, err)
	b, err = backup.ReadMember(ctx, e.store, found, backup.MemberConfig)
	require.NoError(t, err)
	assert.Equal(t, "[DEFAULT]\nid = o2\n", string(b))
	require.NoError(t, e.store.Restore(ctx, runs[0], "fs#1", e.dst))
	b, err = os.ReadFile(filepath.Join(e.dst, "a"))
	require.NoError(t, err)
	assert.Equal(t, "content of a\n", string(b))
}

// plakarCalls counts the plakar calls at the store the log holds, the ones
// of the verb when given.
func plakarCalls(logs, bin, verb string) int {
	n := 0
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "\"run "+bin+" ") && strings.Contains(line, " at @store "+verb) {
			n++
		}
	}
	return n
}

// removeRemote removes the directory of the sftp server the test made its
// store in.
func removeRemote(t *testing.T, u *url.URL, key []byte, hostKey, dir string) {
	t.Helper()
	signer, err := ssh.ParsePrivateKey(key)
	require.NoError(t, err)
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
	require.NoError(t, err)
	port := u.Port()
	if port == "" {
		port = "22"
	}
	conn, err := ssh.Dial("tcp", net.JoinHostPort(u.Hostname(), port), &ssh.ClientConfig{
		User:              u.User.Username(),
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   ssh.FixedHostKey(pk),
		HostKeyAlgorithms: []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, pk.Type()},
		Timeout:           10 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	client, err := sftp.NewClient(conn)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	if _, err := client.Stat(dir); err != nil {
		return
	}
	require.NoError(t, client.RemoveAll(dir))
}
