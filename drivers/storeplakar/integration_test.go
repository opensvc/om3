package storeplakar

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
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
	"sync"
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

// The environment of TestWithTheRealPlakarOverS3: the endpoint of a s3
// server the test may create buckets on, as http://127.0.0.1:9000 for a
// MinIO of the host, and the keys of a user allowed to create, list and
// remove buckets there.
const (
	s3URLEnv       = "OM_TEST_PLAKAR_S3_URL"
	s3AccessKeyEnv = "OM_TEST_PLAKAR_S3_ACCESS_KEY"
	s3SecretKeyEnv = "OM_TEST_PLAKAR_S3_SECRET_KEY"

	// s3TestRegion is the region the test signs its own requests for,
	// the one a s3 server has unless configured otherwise.
	s3TestRegion = "us-east-1"
)

// s3Admin is the client the s3 test looks at its buckets and removes them
// with: the driver has none, plakar being its only way to a store.
type s3Admin struct {
	endpoint  *url.URL
	accessKey string
	secretKey string
}

// do sends one request signed as the version 4 of the s3 signature asks,
// and returns the status and the body of the answer.
func (a s3Admin) do(t *testing.T, method, p string, query url.Values, body []byte) (int, []byte) {
	t.Helper()
	hmacOf := func(key []byte, s string) []byte {
		h := hmac.New(sha256.New, key)
		h.Write([]byte(s))
		return h.Sum(nil)
	}
	now := time.Now().UTC()
	date, day := now.Format("20060102T150405Z"), now.Format("20060102")
	sum := sha256.Sum256(body)
	payload := hex.EncodeToString(sum[:])
	// The escape of a query is the one of a form, a space but for.
	rawQuery := strings.ReplaceAll(query.Encode(), "+", "%20")
	uri := (&url.URL{Path: p}).EscapedPath()
	canonical := strings.Join([]string{
		method, uri, rawQuery,
		"host:" + a.endpoint.Host, "x-amz-content-sha256:" + payload, "x-amz-date:" + date, "",
		"host;x-amz-content-sha256;x-amz-date", payload,
	}, "\n")
	scope := day + "/" + s3TestRegion + "/s3/aws4_request"
	hashed := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + date + "\n" + scope + "\n" + hex.EncodeToString(hashed[:])
	key := hmacOf(hmacOf(hmacOf(hmacOf([]byte("AWS4"+a.secretKey), day), s3TestRegion), "s3"), "aws4_request")
	u := *a.endpoint
	u.Path, u.RawPath, u.RawQuery = p, uri, rawQuery
	req, err := http.NewRequest(method, u.String(), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("x-amz-content-sha256", payload)
	req.Header.Set("x-amz-date", date)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+a.accessKey+"/"+scope+", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(hmacOf(key, toSign)))
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, b
}

// keys returns the keys of the objects of the bucket under the prefix, and
// whether the bucket exists.
func (a s3Admin) keys(t *testing.T, bucket, prefix string) ([]string, bool) {
	t.Helper()
	l := make([]string, 0)
	token := ""
	for {
		query := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			query.Set("continuation-token", token)
		}
		status, b := a.do(t, http.MethodGet, "/"+bucket, query, nil)
		if status == http.StatusNotFound {
			return nil, false
		}
		require.Equal(t, http.StatusOK, status, string(b))
		var listing struct {
			Contents []struct {
				Key string
			}
			NextContinuationToken string
		}
		require.NoError(t, xml.Unmarshal(b, &listing))
		for _, c := range listing.Contents {
			l = append(l, c.Key)
		}
		if token = listing.NextContinuationToken; token == "" {
			return l, true
		}
	}
}

func (a s3Admin) makeBucket(t *testing.T, bucket string) {
	t.Helper()
	status, b := a.do(t, http.MethodPut, "/"+bucket, nil, nil)
	require.Equal(t, http.StatusOK, status, string(b))
}

// removeBucket removes the bucket and its objects, when it exists.
func (a s3Admin) removeBucket(t *testing.T, bucket string) {
	t.Helper()
	keys, ok := a.keys(t, bucket, "")
	if !ok {
		return
	}
	for _, key := range keys {
		status, b := a.do(t, http.MethodDelete, "/"+bucket+"/"+key, nil, nil)
		require.Equal(t, http.StatusNoContent, status, string(b))
	}
	status, b := a.do(t, http.MethodDelete, "/"+bucket, nil, nil)
	require.Equal(t, http.StatusNoContent, status, string(b))
}

// TestWithTheRealPlakarOverS3 runs the driver through the s3 plugin an
// operator installed, against a s3 server, with the keys of a sec, and pins
// what the fake cannot: the answers of the plugin the driver judges a
// location holding no repository on, a bucket missing and a bucket holding
// no CONFIG, the store created by its first write only, the id of its
// repository read back, the region and the TLS of the store as the plugin
// reads them, and one repository left by the first writes made at once.
//
// It needs plakar with its s3 plugin and the variables above, and removes
// the buckets it creates.
func TestWithTheRealPlakarOverS3(t *testing.T) {
	endpoint := os.Getenv(s3URLEnv)
	if endpoint == "" {
		t.Skip("no " + s3URLEnv)
	}
	bin, err := exec.LookPath("plakar")
	if err != nil {
		t.Skip("no plakar in the PATH")
	}
	if l, _ := filepath.Glob(filepath.Join(defaultDataDir(), "plugins", "*", "s3_*")); len(l) == 0 {
		t.Skip("no s3 plugin of plakar in " + defaultDataDir())
	}
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	require.NotEmpty(t, u.Host, s3URLEnv)
	admin := s3Admin{endpoint: u, accessKey: os.Getenv(s3AccessKeyEnv), secretKey: os.Getenv(s3SecretKeyEnv)}
	require.NotEmpty(t, admin.accessKey, s3AccessKeyEnv)
	require.NotEmpty(t, admin.secretKey, s3SecretKeyEnv)

	e := newTestEnv(t)
	dataDir = defaultDataDir
	disableSecurityCheck = runDisableSecurityCheck
	warmUp = runWarmUp
	getPath = func(string) string { return bin }
	t.Cleanup(func() { getPath = capabilities.GetPath })
	om := filepath.Join(t.TempDir(), "om")
	require.NoError(t, os.WriteFile(om, []byte("#!/bin/sh\necho "+passphrase+"\n"), 0755))
	t.Setenv(executable.ExecutableKey, om)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	p, err := naming.ParsePath("system/sec/backup")
	require.NoError(t, err)
	sec, err := object.NewSec(p)
	require.NoError(t, err)
	require.NoError(t, sec.AddKey("access_key", []byte(admin.accessKey)))
	require.NoError(t, sec.AddKey("secret_key", []byte(admin.secretKey)))

	level := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(level) })
	var logs bytes.Buffer
	log := plog.NewLogger(zerolog.New(zerolog.SyncWriter(&logs)).Level(zerolog.TraceLevel))
	plain := u.Scheme == "http"
	// named is the store of a name at a bucket, with a prefix or not,
	// reached as the endpoint is served.
	named := func(name, location string, keywords ...string) *T {
		config := "[store#" + name + "]\ntype = plakar\nurl = s3://" + u.Host + "/" + location + "\npassphrase = from system/sec/backup key passphrase\naccess_key = from system/sec/backup key access_key\nsecret_key = from system/sec/backup key secret_key\n"
		if plain {
			config += "disable_tls = true\n"
		}
		config += strings.Join(keywords, "\n") + "\n"
		cfg, err := xconfig.NewObject("", []byte(config))
		require.NoError(t, err)
		drv := backup.GetDriver("plakar")
		drv.SetName(name)
		drv.SetConfig(cfg)
		s := drv.(*T)
		s.SetLoggerForTest(log)
		return s
	}
	newBucket := func() string {
		bucket := "plakar-test-" + uuid.NewString()[:8]
		t.Cleanup(func() { admin.removeBucket(t, bucket) })
		return bucket
	}
	require.NoError(t, os.WriteFile(filepath.Join(e.src, "a"), []byte("content of a\n"), 0644))

	// A server answering in plain http is not reached without TLS by
	// itself, and the error says what to set.
	missing := newBucket()
	if plain {
		cfg, err := xconfig.NewObject("", []byte("[store#lab]\ntype = plakar\nurl = s3://"+u.Host+"/"+missing+"\npassphrase = from system/sec/backup key passphrase\naccess_key = from system/sec/backup key access_key\nsecret_key = from system/sec/backup key secret_key\n"))
		require.NoError(t, err)
		drv := backup.GetDriver("plakar")
		drv.SetName("lab")
		drv.SetConfig(cfg)
		err = drv.(*T).Backup(ctx, run1, "fs#1", e.src, e.src, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), plainHTTPHint)
		assert.NotErrorIs(t, err, errNoRepository)
	}

	// No read creates the store, nor the marker of a run, nor the bucket:
	// each fails on the answer of the s3 package that there is no bucket.
	e.store = named("lab", missing)
	assert.False(t, e.store.IsLocal())
	_, err = e.store.Runs(ctx, backup.RunFilter{})
	require.ErrorIs(t, err, errNoRepository, "the s3 backend of the plugins installed is found, and the keys of the sec sign")
	assert.Contains(t, err.Error(), "bucket does not exist")
	require.ErrorIs(t, e.store.Check(ctx, backup.RunFilter{}), errNoRepository)
	require.ErrorIs(t, e.store.Delete(ctx, []backup.Run{run1}), errNoRepository)
	require.ErrorIs(t, e.store.Complete(ctx, run1), errNoRepository)
	assert.NotContains(t, logs.String(), "at @store create")
	_, ok := admin.keys(t, missing, "")
	require.False(t, ok, "no bucket is made by a read")

	// Keys the server refuses are not a location holding no repository.
	require.NoError(t, sec.AddKey("refused", []byte("not-the-secret-key")))
	err = named("lab", missing, "secret_key = from system/sec/backup key refused").Backup(ctx, run1, "fs#1", e.src, e.src, nil)
	require.Error(t, err)
	assert.NotErrorIs(t, err, errNoRepository)
	t.Logf("keys refused: %s", err)
	_, ok = admin.keys(t, missing, "")
	require.False(t, ok, "no bucket is made with keys refused")

	// The first write creates the bucket and the store, and reads the id
	// of the repository back after the create and after the write.
	e.store = named("lab", missing)
	logs.Reset()
	require.NoError(t, e.store.Backup(ctx, run1, "fs#1", e.src, e.src, nil))
	assert.Equal(t, 1, plakarCalls(logs.String(), bin, "create"))
	assert.Equal(t, 3, plakarCalls(logs.String(), bin, "info"), "the id is read under the lock of the creation, after the create and after the write")
	keys, ok := admin.keys(t, missing, "")
	require.True(t, ok)
	assert.Contains(t, keys, "CONFIG")
	id, err := e.store.repositoryID(ctx)
	require.NoError(t, err)
	_, err = uuid.Parse(id)
	require.NoError(t, err, "the id of the repository is the one plakar info says: %s", id)

	// A bucket holding nothing, and a prefix of a bucket holding a store,
	// are locations holding no repository, created by the first write, here
	// a stream, with the region of the server set.
	empty := newBucket()
	admin.makeBucket(t, empty)
	for _, location := range []string{empty, missing + "/sub/store"} {
		s := named("lab", location, "region = "+s3TestRegion)
		_, err = s.Runs(ctx, backup.RunFilter{})
		require.ErrorIs(t, err, errNoRepository, location)
		assert.Contains(t, err.Error(), "The specified key does not exist.", location)
		require.ErrorIs(t, s.Complete(ctx, run1), errNoRepository, location)
		logs.Reset()
		require.NoError(t, s.BackupStream(ctx, run1, backup.MemberConfig, strings.NewReader("[DEFAULT]\nid = o1\n")), location)
		assert.Equal(t, 1, plakarCalls(logs.String(), bin, "create"), location)
		other, err := s.repositoryID(ctx)
		require.NoError(t, err, location)
		assert.NotEqual(t, id, other, "a repository of its own: %s", location)
	}
	keys, _ = admin.keys(t, missing, "sub/store/")
	assert.Contains(t, keys, "sub/store/CONFIG")

	// A region the server is not of.
	_, err = named("lab", missing, "region = eu-west-3").Runs(ctx, backup.RunFilter{})
	t.Logf("another region than the one of the server: %v", err)

	// The run is listed, restored, read, checked and deleted.
	e.store = named("lab", missing)
	require.NoError(t, e.store.BackupStream(ctx, run1, backup.MemberConfig, strings.NewReader("[DEFAULT]\nid = o1\n")))
	require.NoError(t, e.store.Complete(ctx, run1))
	second := run1
	second.ID, second.ObjectID, second.Path = "r2", "o2", "test/svc/app2"
	require.NoError(t, e.store.Backup(ctx, second, "fs#1", e.src, e.src, nil))
	require.NoError(t, e.store.Complete(ctx, second))
	runs, err := e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 2)
	found, err := backup.FindRun(ctx, e.store, backup.RunFilter{ID: run1.ID})
	require.NoError(t, err)
	assert.False(t, found.CompletedAt.IsZero())
	assert.Equal(t, []string{"fs#1", backup.MemberConfig}, found.MemberNames())
	require.NoError(t, e.store.Restore(ctx, found, "fs#1", e.dst))
	b, err := os.ReadFile(filepath.Join(e.dst, "a"))
	require.NoError(t, err)
	assert.Equal(t, "content of a\n", string(b))
	b, err = backup.ReadMember(ctx, e.store, found, backup.MemberConfig)
	require.NoError(t, err)
	assert.Equal(t, "[DEFAULT]\nid = o1\n", string(b))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{ID: run1.ID}))
	require.NoError(t, e.store.Check(ctx, backup.RunFilter{}))
	logs.Reset()
	require.NoError(t, e.store.Delete(ctx, []backup.Run{found}))
	assert.Contains(t, logs.String(), "at @store maintenance")
	assert.NotContains(t, logs.String(), "maintenance skipped")
	runs, err = e.store.Runs(ctx, backup.RunFilter{})
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, second.ID, runs[0].ID)

	// The first writes made at once leave one repository: the ones of two
	// objects of the node, sharing the lock of the creation, create once,
	// and the one of another node, here a store of another name, either
	// finds the location taken or the store created, or puts a CONFIG of
	// its own, the s3 package looking for one then putting its own. A
	// writer whose repository was replaced meanwhile fails, saying so, and
	// no other failure is one of a race.
	third := run1
	third.ID, third.ObjectID, third.Path = "r3", "o3", "test/svc/app3"
	for round := 1; round <= s3RaceRounds; round++ {
		location := newBucket()
		names := []string{"lab", "lab", "lab2"}
		if round%2 == 0 {
			// Three nodes, none sharing a lock, in a prefix of a
			// bucket there already.
			admin.makeBucket(t, location)
			location += "/store"
			names = []string{"lab", "lab2", "lab3"}
		}
		writers := make([]*T, len(names))
		for i, name := range names {
			writers[i] = named(name, location)
		}
		logs.Reset()
		errs := make([]error, len(writers))
		var wg sync.WaitGroup
		for i, run := range []backup.Run{run1, second, third} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if i == 1 {
					errs[i] = writers[i].BackupStream(ctx, run, backup.MemberConfig, strings.NewReader("[DEFAULT]\nid = o2\n"))
					return
				}
				errs[i] = writers[i].Backup(ctx, run, "fs#1", e.src, e.src, nil)
			}()
		}
		wg.Wait()
		creates := plakarCalls(logs.String(), bin, "create")
		written, replaced := 0, 0
		for i, err := range errs {
			switch {
			case err == nil:
				written++
			case strings.Contains(err.Error(), "created by another node at the same time"):
				replaced++
			default:
				assert.NoError(t, err, "round %d, writer %d", round, i)
			}
		}
		ids := make(map[string]bool)
		for _, s := range writers {
			id, err := s.repositoryID(ctx)
			require.NoError(t, err)
			ids[id] = true
		}
		taken := strings.Count(logs.String(), "stdout: "+bin+": bucket already initialized")
		t.Logf("round %d, %v: %d creates, %d of them finding the location taken, %d writes done, %d refused as written into a repository replaced", round, names, creates, taken, written, replaced)
		require.Len(t, ids, 1, "one repository, the same for every writer")
		assert.GreaterOrEqual(t, creates, 1)
		assert.LessOrEqual(t, creates, len(ids)+len(names)-1)
		if replaced == 0 {
			assert.Equal(t, len(writers), written)
		}
		// The runs written are read with the key of the repository
		// left, and the next write is one plakar call.
		runs, err := writers[0].Runs(ctx, backup.RunFilter{Incomplete: true})
		require.NoError(t, err, "round %d", round)
		assert.GreaterOrEqual(t, len(runs), written, "round %d", round)
		require.NoError(t, writers[0].Check(ctx, backup.RunFilter{}), "round %d", round)
		logs.Reset()
		require.NoError(t, writers[2].Backup(ctx, third, "fs#2", e.src, e.src, nil))
		assert.Equal(t, 1, plakarCalls(logs.String(), bin, ""), logs.String())
	}
}

// s3RaceRounds is how many times the s3 test races its first writers, each
// time at a new location.
const s3RaceRounds = 6
