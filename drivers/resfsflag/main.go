package resfsflag

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kballard/go-shellquote"
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/confined"
	"github.com/opensvc/om3/v3/util/hostname"
)

// T is the driver structure.
type T struct {
	resource.T
	resource.Restart
	resource.SSH
	Path     naming.Path `json:"path"`
	Nodes    []string    `json:"nodes"`
	Topology topology.T  `json:"topology"`
	lazyFile string
	lazyDir  string
}

func New() resource.Driver {
	return &T{}
}

func (t *T) Abort(ctx context.Context) bool {
	if t.Topology == topology.Flex {
		return false
	}
	if len(t.Nodes) <= 1 {
		return false
	}
	if t.Standby {
		return false
	}
	if t.Path.Kind == naming.KindVol {
		// volumes are enslaved to their consumer services
		return false
	}
	test := func(n string) bool {
		client, err := t.NewSSHClient(n)
		if err != nil {
			t.Log().Warnf("abort? peer %s: new ssh client failed: %s", n, err)
			return false
		}
		defer client.Close()
		session, err := client.NewSession()
		if err != nil {
			t.Log().Warnf("abort? peer %s: new ssh session failed: %s", n, err)
			return false
		}
		defer session.Close()
		var b bytes.Buffer
		session.Stdout = &b
		// The peer runs the command through its login shell: the path,
		// which holds the resource id, is quoted for it. A link is not
		// a flag file, whatever it leads to.
		quoted := shellquote.Join(t.file())
		err = session.Run("test -f " + quoted + " && test ! -h " + quoted)
		if err == nil {
			return true
		}
		ee := err.(*ssh.ExitError)
		ec := ee.Waitmsg.ExitStatus()
		return ec == 0
	}
	hn := hostname.Hostname()
	for _, n := range t.Nodes {
		if n == hn {
			continue
		}
		if test(n) {
			t.Log().Infof("abort! already up on peer %s", n)
			return true
		}
	}
	return false
}

// Start the Resource
func (t *T) Start(ctx context.Context) error {
	if t.file() == "" {
		return fmt.Errorf("empty file path")
	}
	r, err := t.openDir(true)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	name := filepath.Base(t.file())
	f, err := r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		if ok, err := confined.TrustedFile(r, name); err != nil {
			return fmt.Errorf("%s: %w: remove it", t.file(), err)
		} else if ok {
			t.Log().Infof("flag file %s is already installed", t.file())
			return nil
		}
	}
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	t.Log().Infof("installed flag file %s", t.file())
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stop()
	})
	return nil
}

// Stop the Resource
func (t *T) Stop(ctx context.Context) error {
	if t.file() == "" {
		return fmt.Errorf("empty file path")
	}
	return t.stop()
}

// stop removes the flag file. A flag file the node did not make, as one in
// a directory of another user, is not the flag of the resource: it is left,
// and the resource is down already.
func (t *T) stop() error {
	up, err := t.isUp()
	if err != nil {
		t.Log().Warnf("%s: not a flag file of the node: nothing to uninstall", err)
		return nil
	}
	if !up {
		t.Log().Infof("flag file %s is already uninstalled", t.file())
		return nil
	}
	r, err := t.openDir(false)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	t.Log().Infof("uninstall flag file %s", t.file())
	return r.Remove(filepath.Base(t.file()))
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.file()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status(ctx context.Context) status.T {
	if t.file() == "" {
		t.StatusLog().Error("Empty file path")
		return status.NotApplicable
	}
	up, err := t.isUp()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	if up {
		return status.Up
	}
	return status.Down
}

// ProvisionAsLeader implement ProvisionAsLeader for T, this allows fsflag resources
// to have a provision/unprovision call state
func (t *T) ProvisionAsLeader(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// openDir opens the directory of the flag file, under the base directory,
// made with create. Every directory under the parent of the base directory,
// as /dev/shm, must be the node's: owned by root, and writable by root alone.
// A user making the base directory before the node does, or writing in it,
// would otherwise have the node create its flag through a link of theirs,
// truncating any file, or forge the flag of an instance up. A directory of
// another user is refused, never taken over: an admin removes it.
func (t *T) openDir(create bool) (*os.Root, error) {
	anchor := filepath.Dir(t.baseDir())
	rel, err := filepath.Rel(anchor, t.dir())
	if err != nil {
		return nil, err
	}
	return confined.OpenDirTrusted(anchor, rel, create, 0o755, func(p string, mode fs.FileMode) {
		t.Log().Warnf("%s was writable by others (%s): made 0755", p, mode)
	})
}

// isUp says the flag file is installed: a regular file of root in a
// directory tree of root. A file of another user, a link, or a file in a
// directory of another user is not a flag of the node, and the error says
// why.
func (t *T) isUp() (bool, error) {
	r, err := t.openDir(false)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer func() { _ = r.Close() }()
	ok, err := confined.TrustedFile(r, filepath.Base(t.file()))
	if err != nil {
		return false, fmt.Errorf("%s: %w", t.file(), err)
	}
	return ok, nil
}

func (t *T) file() string {
	if t.lazyFile != "" {
		return t.lazyFile
	}
	if t.dir() == "" {
		return ""
	}
	p := fmt.Sprintf("%s/%s.flag", t.dir(), t.ResourceID)
	t.lazyFile = filepath.FromSlash(p)
	return t.lazyFile
}

// MakeBaseDir makes the base directory of the flag files of the node, as
// /dev/shm/opensvc, owned by root and writable by root alone, as the daemon
// does at start: a user can no longer make it first, which would have the
// fs.flag resources of the node refuse to start until an admin removes it.
// A directory of root writable by others is given 0755, and told to warn.
func MakeBaseDir(warn func(format string, args ...any)) error {
	p := nodeBaseDir()
	if p == "" {
		return nil
	}
	return confined.MkdirTrusted(filepath.Dir(p), filepath.Base(p), 0o755, func(p string, mode fs.FileMode) {
		warn("%s was writable by others (%s): made 0755", p, mode)
	})
}

// tmpBase is the base directory of the flag files on linux, in the tmpfs
// every user can write in: the flags go with a reboot, as the instances
// they tell up.
var tmpBase = filepath.FromSlash("/dev/shm/opensvc")

func tmpBaseDir() string {
	return tmpBase
}

func (t *T) dir() string {
	var p string
	if t.lazyDir != "" {
		return t.lazyDir
	}
	if t.Path.Namespace != naming.NsRoot {
		p = fmt.Sprintf("%s/%s/%s/%s", t.baseDir(), t.Path.Namespace, t.Path.Kind, t.Path.Name)
	} else {
		p = fmt.Sprintf("%s/%s/%s", t.baseDir(), t.Path.Kind, t.Path.Name)
	}
	t.lazyDir = filepath.FromSlash(p)
	return t.lazyDir
}

func main() {
	r := &T{}
	if err := resource.NewLoader(os.Stdin).Load(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	resource.Action(context.TODO(), r)
}
