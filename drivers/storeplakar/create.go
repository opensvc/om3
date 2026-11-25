package storeplakar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	ulock "github.com/opensvc/om3/v3/util/lock"
)

const (
	// exitRepoNotFound is what plakar exits with when it cannot open the
	// repository of the store, whatever the reason: a location holding
	// none, a server not reached, a login refused.
	exitRepoNotFound = 66

	// openFailedMessage opens, after the name of the program, the line
	// plakar says the reason on, closing it.
	openFailedMessage = "failed to open the repository at "

	// createLockName is the file of the configuration directory of the
	// store the creation of its repository is serialized with on the
	// node.
	createLockName = "create.lock"

	// createLockTimeout is how long a write waits for the creation another
	// one runs: a create and the read of the repository id, a second
	// each, behind a slow location.
	createLockTimeout = 2 * time.Minute
)

var (
	// errNoRepository is the definite answer of plakar that the location
	// is reached and holds no repository: the only one a write creates
	// the store on.
	errNoRepository = errors.New("the location holds no repository")

	// absentMessages are, by scheme, the reasons plakar 1.1.8 and its
	// packages close the openFailedMessage line with when the location is
	// reached, with the credentials of the store, and holds no CONFIG:
	//
	//   - fs: the open of <dir>/CONFIG, as the os says it.
	//   - sftp, package 1.1.11: the server answering no such file to the
	//     open of <dir>/CONFIG, the directory missing or not.
	//   - s3, package 1.1.12: the bucket missing, or the server answering
	//     NoSuchKey to the read of <prefix>/CONFIG, as its client words it.
	//
	// Every other reason is a location not judged: a server not reached, a
	// timeout, a login, a host key or a permission refused, a server in
	// trouble. A scheme with no entry is never judged: the http client of a
	// plakar server has no such answer, a server serving a repository it
	// opened itself.
	absentMessages = map[string][]string{
		"fs":   {": no such file or directory"},
		"sftp": {": file does not exist"},
		"s3":   {": bucket does not exist", ": error reading object: The specified key does not exist."},
	}

	// createdSchemes are the schemes of the remote locations the driver
	// creates a store at: the ones whose package creates a repository,
	// refusing a location holding one, and tells a location holding none
	// from one it cannot judge. plakar create at a plakar server writes
	// nothing and exits 0.
	createdSchemes = map[string]bool{
		"sftp": true,
		"s3":   true,
	}

	// alreadyCreatedREs match what plakar create says, exiting 1, of a
	// location another create got to first: the directory of a fs or sftp
	// location holding something, its locks directory made meanwhile, the
	// CONFIG of a s3 location found. The line opens with the name plakar
	// was run by, its path, and is printed on its standard output, as
	// plakar does of what a command fails on, where it says a repository
	// not opened on its standard error.
	alreadyCreatedREs = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^.+: directory .+ is not empty$`),
		regexp.MustCompile(`(?m)^.+: mkdir .+/locks: file exists$`),
		regexp.MustCompile(`(?m)^.+: bucket already initialized$`),
	}

	// repositoryIDRE matches the line of plakar info telling the id of the
	// repository, drawn by its create and held by its CONFIG.
	repositoryIDRE = regexp.MustCompile(`(?m)^RepositoryID: (\S+)$`)

	// createdTries and createdDelay bound the wait for the CONFIG of a
	// store another node is creating, written last, after the 513
	// directories of a repository. Replaced by the tests.
	createdTries = 10
	createdDelay = time.Second

	// creationMus are the locks of the creations of this process, by lock
	// file: a fcntl lock is the one of a process, which two of its
	// goroutines both hold.
	creationMus sync.Map
)

// noRepository tells whether plakar, exiting with code and saying stderr,
// answered that the location of the scheme is reached and holds no
// repository.
func noRepository(scheme string, code int, stderr []byte) bool {
	if code != exitRepoNotFound {
		return false
	}
	for _, line := range strings.Split(string(stderr), "\n") {
		_, reason, ok := strings.Cut(strings.TrimRight(line, "\r"), ": "+openFailedMessage)
		if !ok {
			continue
		}
		for _, msg := range absentMessages[scheme] {
			if strings.HasSuffix(reason, msg) {
				return true
			}
		}
	}
	return false
}

// alreadyCreated tells whether a failed plakar create said, in its output
// out, that the location holds something already.
func alreadyCreated(out []byte) bool {
	out = bytes.ReplaceAll(out, []byte("\r\n"), []byte("\n"))
	for _, re := range alreadyCreatedREs {
		if re.Match(out) {
			return true
		}
	}
	return false
}

// createError is callError for a create, with what plakar printed on its
// standard output, where it says why a create failed.
func (t *T) createError(cmd *command.T, err error) error {
	if out := strings.TrimSpace(string(cmd.Stdout())); out != "" {
		return fmt.Errorf("%w: %s", t.callError("create", cmd, err), out)
	}
	return t.callError("create", cmd, err)
}

// storeError is callError for a call at the store, an errNoRepository when
// plakar answered the location holds no repository.
func (t *T) storeError(verb string, cmd *command.T, err error) error {
	e := t.callError(verb, cmd, err)
	location, _, lerr := t.location()
	if lerr != nil || !noRepository(scheme(location), cmd.NormalizedExitCode(), cmd.Stderr()) {
		return e
	}
	return fmt.Errorf("%w: %w", errNoRepository, e)
}

// creates tells whether the store is at a remote location the driver
// creates a store at.
func (t *T) creates() bool {
	location, dir, err := t.location()
	return err == nil && dir == "" && createdSchemes[scheme(location)]
}

// setPresent records a call of this driver found the repository.
func (t *T) setPresent() {
	t.createMu.Lock()
	defer t.createMu.Unlock()
	t.present = true
}

func (t *T) isPresent() bool {
	t.createMu.Lock()
	defer t.createMu.Unlock()
	return t.present
}

func (t *T) setCreated(id string) {
	t.createMu.Lock()
	defer t.createMu.Unlock()
	t.created = id
}

// takeCreated returns the id of the repository this driver created, once.
func (t *T) takeCreated() string {
	t.createMu.Lock()
	defer t.createMu.Unlock()
	id := t.created
	t.created = ""
	return id
}

// lockCreation serializes the creation of the store on the node: two
// writes finding no store at once, as the first runs of two objects, would
// each create one, every create drawing its own key, and the runs written
// under the key of the CONFIG replaced would not be read again. The lock is
// in the configuration directory of the store, the same for every object of
// the node using it.
func (t *T) lockCreation() (func(), error) {
	d, err := t.dirs()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(d.config, createLockName)
	v, _ := creationMus.LoadOrStore(p, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	lock := flock.New(p, "", fcntllock.New)
	if err := lock.Lock(createLockTimeout, "create"); err != nil {
		mu.Unlock()
		return nil, fmt.Errorf("lock the creation of the store: %w", ulock.Explain(lock, createLockTimeout, err))
	}
	return func() {
		_ = lock.UnLock()
		mu.Unlock()
	}, nil
}

// repositoryID returns the id of the repository of the store, as plakar
// info tells it: an errNoRepository when the location holds none.
func (t *T) repositoryID(ctx context.Context) (string, error) {
	out, err := t.run(ctx, zerolog.TraceLevel, "info")
	if err != nil {
		return "", err
	}
	m := repositoryIDRE.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("plakar info: no RepositoryID line")
	}
	return string(m[1]), nil
}

// awaitRepository returns the id of the repository of the store, read again
// while the location holds none, tries times at most.
func (t *T) awaitRepository(ctx context.Context, tries int) (string, error) {
	for i := 1; ; i++ {
		id, err := t.repositoryID(ctx)
		if err == nil || !errors.Is(err, errNoRepository) || i >= tries {
			return id, err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(createdDelay):
		}
	}
}

// awaitConfig waits for the CONFIG of the store of the node another node
// is creating in a directory they share.
func awaitConfig(ctx context.Context, dir string) error {
	for i := 1; ; i++ {
		_, err := os.Stat(filepath.Join(dir, "CONFIG"))
		if err == nil || !errors.Is(err, os.ErrNotExist) || i >= createdTries {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(createdDelay):
		}
	}
}

// create creates the store a write found missing: dir is the directory of a
// store of the node, found missing or empty, and is empty for a remote
// store plakar answered an errNoRepository for.
//
// The creation is serialized on the node, and the store looked for again
// once the lock is held: the write that waited for the lock finds the store
// the other one created.
//
// The nodes sharing a store are not serialized. A create finding the
// location taken meanwhile is no failure: the store is the one of the other
// node, waited for until its CONFIG, written last, is there, and never
// written over. A create failing otherwise creates nothing more, and fails
// the write unless the repository is there after all, as after a create
// that lost on an error not telling so.
//
// The packages of a directory and of a sftp location make one repository
// whatever the creates racing, on the directory only one of them makes. The
// s3 package looks for a CONFIG then puts its own, so two creates a few
// milliseconds apart both succeed and the last CONFIG stays: the id of the
// repository of a remote store is read back, a second later, and kept for
// the write to check once done, see checkCreated.
func (t *T) create(ctx context.Context, dir string) error {
	unlock, err := t.lockCreation()
	if err != nil {
		return err
	}
	defer unlock()
	if dir != "" {
		if v, err := storeExists(dir); err != nil || v {
			return err
		}
	} else {
		switch id, err := t.repositoryID(ctx); {
		case err == nil:
			t.setCreated(id)
			return nil
		case !errors.Is(err, errNoRepository):
			return err
		}
	}
	t.Log().Infof("no store at %s: create it", t.Get("url"))
	cmd, err := t.commandAt(ctx, zerolog.InfoLevel, nil, "create")
	if err != nil {
		return err
	}
	var cerr error
	taken := false
	if err := cmd.Run(); err != nil {
		cerr = t.createError(cmd, err)
		if taken = alreadyCreated(cmd.Stdout()); taken {
			t.Log().Infof("the location of the store was taken while it was created: use the store created there")
		}
	}
	if dir != "" {
		switch {
		case cerr == nil:
			return nil
		case !taken:
			return cerr
		}
		if err := awaitConfig(ctx, dir); err != nil {
			return fmt.Errorf("the store is not created, and %s holds no store: %w", dir, cerr)
		}
		return nil
	}
	tries := 1
	if taken {
		tries = createdTries
	}
	id, err := t.awaitRepository(ctx, tries)
	switch {
	case err == nil:
		if cerr != nil && !taken {
			t.Log().Infof("the store is there after the create failed: use it as it is")
		}
		t.setCreated(id)
		return nil
	case taken && errors.Is(err, errNoRepository):
		return fmt.Errorf("the store is not created, and its location holds no store: %w", cerr)
	case cerr != nil:
		return cerr
	default:
		return fmt.Errorf("the store is created, its repository is not read back: %w", err)
	}
}

// ensureRemote creates the remote store a write is about to find missing,
// for a write that cannot be run again once it failed, as the one of a
// stream read once: the repository is looked for before, unless a call of
// this driver found it already, as the backup of a head did.
func (t *T) ensureRemote(ctx context.Context) error {
	if !t.creates() || t.isPresent() {
		return nil
	}
	_, err := t.repositoryID(ctx)
	if errors.Is(err, errNoRepository) {
		return t.create(ctx, "")
	}
	return err
}

// checkCreated reads the id of the repository again after the write that
// created the store, and fails the write when it is no longer the one read
// after the create: another node created the store at the same time, at a
// location whose package does not make one repository of two creates, and
// its CONFIG replaced the one this write may have read its key from. What
// was written is then not read again. The store is left as it is: the next
// run writes into the repository that stayed.
func (t *T) checkCreated(ctx context.Context) error {
	created := t.takeCreated()
	if created == "" {
		return nil
	}
	id, err := t.repositoryID(ctx)
	if err != nil {
		return fmt.Errorf("read the repository of the store back after its first write: %w", err)
	}
	if id != created {
		return fmt.Errorf("store %s: the repository %s was replaced by %s, created by another node at the same time, while its first run was written: what was written may not be read again, run again", t.Ref(), created, id)
	}
	return nil
}
