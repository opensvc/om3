package storeplakar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/backup"
)

// repoHook is the hook of the fake plakar holding a repository, the file
// repo of its directory, its content the id: a call at a store with no
// repository says the content of the file absent and exits with the one of
// absent.status, a create makes the repository, or says the content of the
// file taken when there is one, on its standard output and under the path
// it was run by as plakar does, and an info tells the id. A <verb>.sh file
// is sourced first, to answer otherwise.
const repoHook = `said() { sed "s|^plakar: |$0: |" "$1"; }
verb=
seen=0
for a in "$@"; do
	if [ $seen = 1 ]; then verb=$a; break; fi
	if [ "$a" = @store ]; then seen=1; fi
done
if [ -n "$verb" ] && [ -f "$dir/$verb.sh" ]; then . "$dir/$verb.sh"; fi
case $verb in
create)
	if [ -f "$dir/repo" ]; then said "$dir/taken"; exit 1; fi
	sleep 0.3
	echo id1 >"$dir/repo"
	exit 0
	;;
"") ;;
*)
	if [ ! -f "$dir/repo" ]; then cat "$dir/absent" >&2; exit $(cat "$dir/absent.status"); fi
	if [ $verb = info ]; then echo "RepositoryID: $(cat "$dir/repo")"; exit 0; fi
	;;
esac
`

// What plakar 1.1.8 says, exiting 66, of a location it reaches holding no
// repository, with its sftp 1.1.11 and s3 1.1.12 packages, and of a
// location it cannot judge, as recorded.
const (
	openHint = "To specify an alternative repository, please use \"plakar at <location> <command>\".\n"

	sftpAbsent     = "plakar: failed to open the repository at sftp://backup@h/data: file does not exist\n" + openHint
	s3AbsentBucket = "plakar: failed to open the repository at s3://h/bucket: bucket does not exist\n" + openHint
	s3AbsentConfig = "plakar: failed to open the repository at s3://h/bucket: error reading object: The specified key does not exist.\n" + openHint
	fsAbsent       = "plakar: failed to open the repository at fs:///srv/plakar: open /srv/plakar/CONFIG: no such file or directory\n" + openHint

	sftpDenied   = "plakar: failed to open the repository at sftp://backup@h/data: permission denied\n" + openHint
	sftpLogin    = "plakar: SSH connection to \"h\" failed: failed to start ssh master: \"/usr/bin/ssh -o BatchMode=yes -l backup -N -f -- h\": exit status 255: \"backup@h: Permission denied (publickey,password,keyboard-interactive).\"\nplakar: failed to open the repository at sftp://backup@h/data: SSH connection to \"h\" failed: failed to start ssh master: \"/usr/bin/ssh -o BatchMode=yes -l backup -N -f -- h\": exit status 255: \"backup@h: Permission denied (publickey,password,keyboard-interactive).\"\n" + openHint
	sftpHostKey  = "plakar: failed to open the repository at sftp://backup@h/data: SSH connection to \"h\" failed: failed to start ssh master: \"/usr/bin/ssh -- h\": exit status 255: \"No ED25519 host key is known for h and you have requested strict checking.\\r\\nHost key verification failed.\"\n" + openHint
	sftpRefused  = "plakar: failed to open the repository at sftp://backup@h/data: SSH connection to \"h\" failed: failed to start ssh master: \"/usr/bin/ssh -- h\": exit status 255: \"ssh: connect to host h port 22: Connection refused\"\n" + openHint
	sftpTimeout  = "plakar: failed to open the repository at sftp://backup@h/data: SSH connection to \"h\" failed: failed to start ssh master: \"/usr/bin/ssh -- h\": exit status 255: \"ssh: connect to host h port 22: Connection timed out\"\n" + openHint
	s3Denied     = "plakar: failed to open the repository at s3://h/bucket: error checking if bucket exists: Access Denied.\n" + openHint
	s3Refused    = "plakar: failed to open the repository at s3://h/bucket: error checking if bucket exists: Head \"https://h/bucket/\": dial tcp 192.0.2.1:443: connect: connection refused\n" + openHint
	s3Trouble    = "plakar: failed to open the repository at s3://h/bucket: error checking if bucket exists: 503 Service Unavailable\n" + openHint
	httpsRefused = "plakar: failed to open the repository at https://h: Get \"https://h/\": dial tcp 192.0.2.1:443: connect: connection refused\n" + openHint

	sftpTaken = "plakar: directory /data is not empty\n"
	s3Taken   = "plakar: bucket already initialized\n"
)

// setRepo makes the fake plakar hold no repository, a call at its store
// saying absent and exiting with status, and its create saying taken of a
// location holding one.
func setRepo(t *testing.T, absent string, status string, taken string) {
	t.Helper()
	setFake(t, "hook", []byte(repoHook))
	setFake(t, "absent", []byte(absent))
	setFake(t, "absent.status", []byte(status))
	setFake(t, "taken", []byte(taken))
}

// verbs returns the verb of every plakar call made at the store.
func verbs(t *testing.T) []string {
	t.Helper()
	l := make([]string, 0)
	for _, argv := range calls(t) {
		for i, a := range argv {
			if a == "@store" && i+1 < len(argv) {
				l = append(l, argv[i+1])
				break
			}
		}
	}
	return l
}

func count(l []string, s string) int {
	n := 0
	for _, e := range l {
		if e == s {
			n++
		}
	}
	return n
}

// shortCreatedWait makes the wait for the store another node creates one
// of the test.
func shortCreatedWait(t *testing.T) {
	t.Helper()
	tries, delay := createdTries, createdDelay
	createdTries, createdDelay = 2, 10*time.Millisecond
	t.Cleanup(func() { createdTries, createdDelay = tries, delay })
}

func (e credsEnv) s3Store(t *testing.T) *T {
	t.Helper()
	return e.store(t, "s3://h/bucket", "access_key = from system/sec/backup key access_key", "secret_key = from system/sec/backup key secret_key")
}

// TestNoRepositoryIsTheDefiniteAnswerOnly pins the lines of plakar read as
// a location reached and holding no repository, of the scheme they are of
// and with the exit code of a repository not opened only.
func TestNoRepositoryIsTheDefiniteAnswerOnly(t *testing.T) {
	for _, tc := range []struct {
		scheme string
		code   int
		stderr string
		want   bool
	}{
		{"sftp", 66, sftpAbsent, true},
		{"s3", 66, s3AbsentBucket, true},
		{"s3", 66, s3AbsentConfig, true},
		{"fs", 66, fsAbsent, true},
		{"sftp", 1, sftpAbsent, false},
		{"sftp", 66, "plakar: file does not exist\n", false},
		{"sftp", 66, sftpDenied, false},
		{"sftp", 66, sftpLogin, false},
		{"sftp", 66, sftpHostKey, false},
		{"sftp", 66, sftpRefused, false},
		{"sftp", 66, sftpTimeout, false},
		{"sftp", 66, s3AbsentBucket, false},
		{"s3", 66, sftpAbsent, false},
		{"s3", 66, s3Denied, false},
		{"s3", 66, s3Refused, false},
		{"s3", 66, s3Trouble, false},
		{"https", 66, httpsRefused, false},
		{"https", 66, strings.ReplaceAll(sftpAbsent, "sftp://backup@h/data", "https://h"), false},
		{"sftp", 66, "", false},
	} {
		assert.Equal(t, tc.want, noRepository(tc.scheme, tc.code, []byte(tc.stderr)), "%s %d %s", tc.scheme, tc.code, tc.stderr)
	}
	assert.True(t, alreadyCreated([]byte(sftpTaken)))
	assert.True(t, alreadyCreated([]byte("/usr/bin/plakar: directory /data/plakar is not empty\n")), "plakar names itself by the path it was run by")
	assert.True(t, alreadyCreated([]byte(s3Taken)))
	assert.True(t, alreadyCreated([]byte("plakar: mkdir /srv/plakar/locks: file exists\n")))
	assert.False(t, alreadyCreated([]byte("plakar: permission denied\n")))
	assert.False(t, alreadyCreated([]byte("plakar: bucket already initialized: object lock is enabled and still holds CONFIG's version history; clear it or use a different bucket or prefix\n")))
	assert.False(t, alreadyCreated([]byte("plakar: check if bucket exists: Access Denied.\n")))
}

// TestRemoteStoreIsCreatedByItsFirstWrite pins the create of a sftp or s3
// store by the write plakar answers holds no repository: the backup of a
// head found it, at no call more for a store there, and is run again, a
// stream, read once, is preceded by a look for the repository unless a call
// of the driver found it. The id of the repository is read after the
// create and after the write.
func TestRemoteStoreIsCreatedByItsFirstWrite(t *testing.T) {
	ctx := context.Background()
	stores := map[string]struct {
		store  func(e credsEnv) *T
		absent []string
		taken  string
	}{
		"sftp": {func(e credsEnv) *T { return e.sftpStore(t) }, []string{sftpAbsent}, sftpTaken},
		"s3":   {func(e credsEnv) *T { return e.s3Store(t) }, []string{s3AbsentBucket, s3AbsentConfig}, s3Taken},
	}
	for name, tc := range stores {
		for _, absent := range tc.absent {
			e := newCredsEnv(t)
			s := tc.store(e)
			setRepo(t, absent, "66", tc.taken)
			require.NoError(t, s.Backup(ctx, run1, "fs#1", e.src, e.src, nil), name)
			assert.Equal(t, []string{"backup", "info", "create", "info", "backup", "info"}, verbs(t), name)
			require.FileExists(t, filepath.Join(fakeDir, "repo"), name)

			// The store is there: a call per write, the driver having
			// found it.
			require.NoError(t, os.Remove(filepath.Join(fakeDir, "calls")))
			require.NoError(t, s.Backup(ctx, run1, "fs#2", e.src, e.src, nil), name)
			require.NoError(t, s.BackupStream(ctx, run1, "config", strings.NewReader("x")), name)
			require.NoError(t, s.Complete(ctx, run1), name)
			assert.Equal(t, []string{"backup", "backup", "backup"}, verbs(t), name)

			// A driver that found nothing yet looks for the repository
			// before the stream, once.
			require.NoError(t, os.Remove(filepath.Join(fakeDir, "calls")))
			s = tc.store(e)
			require.NoError(t, s.BackupStream(ctx, run1, "config", strings.NewReader("x")), name)
			require.NoError(t, s.BackupStream(ctx, run1, "refs.json", strings.NewReader("y")), name)
			assert.Equal(t, []string{"info", "backup", "backup"}, verbs(t), name)

			e = newCredsEnv(t)
			s = tc.store(e)
			setRepo(t, absent, "66", tc.taken)
			require.NoError(t, s.BackupStream(ctx, run1, "config", strings.NewReader("the stream")), name)
			assert.Equal(t, []string{"info", "info", "create", "info", "backup", "info"}, verbs(t), name)
			stdin, err := os.ReadFile(filepath.Join(fakeDir, "stdin.5"))
			require.NoError(t, err)
			assert.Equal(t, "the stream", string(stdin), "%s: the stream is read by the backup only", name)
		}
	}
}

// TestReadsAndMarkersNeverCreateARemoteStore pins the calls of a remote
// store holding no repository that are no write of a member: they fail with
// what plakar says, and none creates the store.
func TestReadsAndMarkersNeverCreateARemoteStore(t *testing.T) {
	ctx := context.Background()
	e := newCredsEnv(t)
	s := e.sftpStore(t)
	setRepo(t, sftpAbsent, "66", sftpTaken)
	run := run1
	run.Members = []backup.Member{{Name: "config", Kind: backup.MemberStream}}
	for name, err := range map[string]error{
		"runs":      func() error { _, err := s.Runs(ctx, backup.RunFilter{}); return err }(),
		"stats":     func() error { _, err := s.ObjectStats(ctx); return err }(),
		"check":     s.Check(ctx, backup.RunFilter{}),
		"check run": s.Check(ctx, backup.RunFilter{ID: "r1"}),
		"restore":   s.Restore(ctx, run, "fs#1", e.dst),
		"cat":       func() error { _, err := s.Cat(ctx, run, "config"); return err }(),
		"delete":    s.Delete(ctx, []backup.Run{run}),
		"complete":  s.Complete(ctx, run),
	} {
		require.ErrorIs(t, err, errNoRepository, name)
		assert.Contains(t, err.Error(), "file does not exist", name)
	}
	l := verbs(t)
	assert.NotEmpty(t, l)
	assert.NotContains(t, l, "create")
	assert.NotContains(t, l, "info")
	assert.NoFileExists(t, filepath.Join(fakeDir, "repo"))
}

// TestOnlyTheAnswerOfNoRepositoryCreates pins the writes failing, with no
// create, on every other failure to open the repository, a location not
// reached, a login or a permission refused, and on the one of a scheme the
// driver does not create at, as a plakar server.
func TestOnlyTheAnswerOfNoRepositoryCreates(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		store  func(e credsEnv) *T
		stderr string
		status string
		probed bool
	}{
		{"sftp permission denied", func(e credsEnv) *T { return e.sftpStore(t) }, sftpDenied, "66", true},
		{"sftp login refused", func(e credsEnv) *T { return e.sftpStore(t) }, sftpLogin, "66", true},
		{"sftp host key", func(e credsEnv) *T { return e.sftpStore(t) }, sftpHostKey, "66", true},
		{"sftp refused", func(e credsEnv) *T { return e.sftpStore(t) }, sftpRefused, "66", true},
		{"sftp timeout", func(e credsEnv) *T { return e.sftpStore(t) }, sftpTimeout, "66", true},
		{"sftp another exit code", func(e credsEnv) *T { return e.sftpStore(t) }, sftpAbsent, "1", true},
		{"s3 access denied", func(e credsEnv) *T { return e.s3Store(t) }, s3Denied, "66", true},
		{"s3 refused", func(e credsEnv) *T { return e.s3Store(t) }, s3Refused, "66", true},
		{"s3 in trouble", func(e credsEnv) *T { return e.s3Store(t) }, s3Trouble, "66", true},
		{"https refused", func(e credsEnv) *T { return e.store(t, "https://h") }, httpsRefused, "66", false},
		{"https saying no file", func(e credsEnv) *T { return e.store(t, "https://h") }, strings.ReplaceAll(sftpAbsent, "sftp://backup@h/data", "https://h"), "66", false},
	} {
		e := newCredsEnv(t)
		s := tc.store(e)
		setRepo(t, tc.stderr, tc.status, sftpTaken)
		require.Error(t, s.Backup(ctx, run1, "fs#1", e.src, e.src, nil), tc.name)
		assert.Equal(t, []string{"backup"}, verbs(t), tc.name)

		require.NoError(t, os.Remove(filepath.Join(fakeDir, "calls")))
		require.Error(t, s.BackupStream(ctx, run1, "config", strings.NewReader("x")), tc.name)
		if tc.probed {
			assert.Equal(t, []string{"info"}, verbs(t), "%s: the stream is not read", tc.name)
		} else {
			assert.Equal(t, []string{"backup"}, verbs(t), "%s: a store the driver does not create is not looked for", tc.name)
		}
		assert.NoFileExists(t, filepath.Join(fakeDir, "repo"), tc.name)
	}
}

// TestALocationTakenMeanwhileIsUsedAsItIs pins the create finding the
// location taken by the create of another node: no failure, the write going
// on in the store of the other node once its repository is there, and a
// failure when none comes, the location holding something else.
func TestALocationTakenMeanwhileIsUsedAsItIs(t *testing.T) {
	ctx := context.Background()
	shortCreatedWait(t)
	for name, tc := range map[string]struct {
		store func(e credsEnv) *T
		taken string
	}{
		"sftp": {func(e credsEnv) *T { return e.sftpStore(t) }, sftpTaken},
		"s3":   {func(e credsEnv) *T { return e.s3Store(t) }, s3Taken},
	} {
		e := newCredsEnv(t)
		s := tc.store(e)
		setRepo(t, sftpAbsent, "66", tc.taken)
		if name == "s3" {
			setFake(t, "absent", []byte(s3AbsentConfig))
		}
		// The other node creates the store while this create runs.
		setFake(t, "create.sh", []byte("echo id9 >\"$dir/repo\"\n"))
		require.NoError(t, s.Backup(ctx, run1, "fs#1", e.src, e.src, nil), name)
		assert.Equal(t, []string{"backup", "info", "create", "info", "backup", "info"}, verbs(t), name)
		b, err := os.ReadFile(filepath.Join(fakeDir, "repo"))
		require.NoError(t, err)
		assert.Equal(t, "id9\n", string(b), "%s: the store of the other node is not written over", name)

		// The location holds something that is no repository.
		e = newCredsEnv(t)
		s = tc.store(e)
		setRepo(t, sftpAbsent, "66", tc.taken)
		if name == "s3" {
			setFake(t, "absent", []byte(s3AbsentConfig))
		}
		setFake(t, "create.sh", []byte("said \"$dir/taken\"\nexit 1\n"))
		err = s.Backup(ctx, run1, "fs#1", e.src, e.src, nil)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "its location holds no store", name)
		assert.Contains(t, err.Error(), ": "+strings.TrimPrefix(strings.TrimSpace(tc.taken), "plakar: "), name)
		assert.Equal(t, []string{"backup", "info", "create", "info", "info"}, verbs(t), name)
	}
}

// TestACreateFailingOtherwiseFailsTheWrite pins the create failing on
// something else than a location taken: the write fails with what the
// create said, unless the repository is there after all.
func TestACreateFailingOtherwiseFailsTheWrite(t *testing.T) {
	ctx := context.Background()
	e := newCredsEnv(t)
	s := e.sftpStore(t)
	setRepo(t, sftpAbsent, "66", sftpTaken)
	setFake(t, "create.sh", []byte("echo \"$0: permission denied\"\nexit 1\n"))
	err := s.Backup(ctx, run1, "fs#1", e.src, e.src, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plakar create: ")
	assert.Contains(t, err.Error(), "permission denied")
	assert.Equal(t, []string{"backup", "info", "create", "info"}, verbs(t))
	assert.NoFileExists(t, filepath.Join(fakeDir, "repo"))

	// The create of another node won on an error not saying so.
	e = newCredsEnv(t)
	s = e.sftpStore(t)
	setRepo(t, sftpAbsent, "66", sftpTaken)
	setFake(t, "create.sh", []byte("echo id9 >\"$dir/repo\"\necho \"$0: sftp: \\\"Failure\\\" (SSH_FX_FAILURE)\"\nexit 1\n"))
	require.NoError(t, s.Backup(ctx, run1, "fs#1", e.src, e.src, nil))
	assert.Equal(t, []string{"backup", "info", "create", "info", "backup", "info"}, verbs(t))
}

// TestARepositoryReplacedUnderTheFirstWriteFailsIt pins the id of the
// repository read after the create and after the write: another one, the
// CONFIG of a create of another node having replaced the one the write read
// its key from, fails the write, the store left as it is.
func TestARepositoryReplacedUnderTheFirstWriteFailsIt(t *testing.T) {
	ctx := context.Background()
	writes := map[string]func(e credsEnv, s *T) error{
		"backup": func(e credsEnv, s *T) error {
			return s.Backup(ctx, run1, "fs#1", e.src, e.src, nil)
		},
		"backup stream": func(e credsEnv, s *T) error {
			return s.BackupStream(ctx, run1, "config", strings.NewReader("x"))
		},
	}
	for name, write := range writes {
		e := newCredsEnv(t)
		s := e.s3Store(t)
		setRepo(t, s3AbsentConfig, "66", s3Taken)
		// The other node puts its CONFIG while the backup runs.
		setFake(t, "backup.sh", []byte("if [ -f \"$dir/repo\" ]; then echo id2 >\"$dir/repo\"; fi\n"))
		err := write(e, s)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "the repository id1 was replaced by id2", name)
		assert.Equal(t, 1, count(verbs(t), "create"), name)
		b, err := os.ReadFile(filepath.Join(fakeDir, "repo"))
		require.NoError(t, err)
		assert.Equal(t, "id2\n", string(b), "%s: the store that stayed is not written over", name)

		// The next write goes to the repository that stayed, unchecked.
		require.NoError(t, os.Remove(filepath.Join(fakeDir, "calls")))
		require.NoError(t, write(e, s), name)
		assert.Equal(t, []string{"backup"}, verbs(t), name)
	}
}

// TestConcurrentFirstWritesCreateOnce pins the creation serialized on the
// node: the first writes of two objects, each with its driver, finding no
// store at once create one, the second finding the store of the first once
// it holds the lock, for a remote store and for a directory.
func TestConcurrentFirstWritesCreateOnce(t *testing.T) {
	ctx := context.Background()
	both := func(a, b *T, src string) {
		t.Helper()
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, s := range []*T{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = s.Backup(ctx, run1, "fs#1", src, src, nil)
			}()
		}
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
	}

	e := newCredsEnv(t)
	setRepo(t, sftpAbsent, "66", sftpTaken)
	both(e.sftpStore(t), e.sftpStore(t), e.src)
	l := verbs(t)
	assert.Equal(t, 1, count(l, "create"), "%v", l)
	assert.Equal(t, 4, count(l, "backup"), "the two backups finding no store, run again: %v", l)

	// A directory: the fake creates it as plakar does, in a while.
	e = newCredsEnv(t)
	setFake(t, "hook", []byte(repoHook))
	setFake(t, "create.sh", []byte("sleep 0.3\nmkdir -p '"+e.storePath()+"'\n: >'"+e.storePath()+"/CONFIG'\necho id1 >\"$dir/repo\"\nexit 0\n"))
	both(e.testEnv.store, newStore(t, "[store#lab]\ntype = plakar\nurl = "+e.storePath()+"\npassphrase = from system/sec/backup key passphrase\n"), e.src)
	l = verbs(t)
	assert.Equal(t, 1, count(l, "create"), "%v", l)
	assert.Equal(t, 2, count(l, "backup"), "%v", l)
}

// TestADirectoryTakenMeanwhileIsUsedAsItIs pins the create of a directory
// another node sharing it got to first: the store is the one of that node,
// waited for until its CONFIG is there, with no plakar call more.
func TestADirectoryTakenMeanwhileIsUsedAsItIs(t *testing.T) {
	ctx := context.Background()
	shortCreatedWait(t)
	e := newTestEnv(t)
	setFake(t, "hook", []byte(repoHook))
	setFake(t, "create.sh", []byte("mkdir -p '"+e.storePath()+"'\n: >'"+e.storePath()+"/CONFIG'\necho id9 >\"$dir/repo\"\necho \"$0: directory "+e.storePath()+" is not empty\"\nexit 1\n"))
	require.NoError(t, e.store.Backup(ctx, run1, "fs#1", e.src, e.src, nil))
	assert.Equal(t, []string{"create", "backup"}, verbs(t))

	// A directory holding something else is not a store.
	e = newTestEnv(t)
	setFake(t, "hook", []byte(repoHook))
	setFake(t, "create.sh", []byte("echo \"$0: directory "+e.storePath()+" is not empty\"\nexit 1\n"))
	err := e.store.Backup(ctx, run1, "fs#1", e.src, e.src, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "holds no store")
	assert.Equal(t, []string{"create"}, verbs(t))
}
