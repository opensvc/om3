package resfsflag

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/file"
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
		err = session.Run("test -f " + t.file())
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
	if t.exists() {
		t.Log().Infof("flag file %s is already installed", t.file())
		return nil
	}
	if err := os.MkdirAll(t.dir(), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", t.dir(), err)
	}
	t.Log().Infof("install flag file %s", t.file())
	if _, err := os.Create(t.file()); err != nil {
		return err
	}
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
	if !t.exists() {
		t.Log().Infof("flag file %s is already uninstalled", t.file())
		return nil
	}
	return t.stop()
}

func (t *T) stop() error {
	p := t.file()
	t.Log().Infof("uninstall flag file %s", p)
	return os.Remove(p)
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
	if t.exists() {
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

func (t *T) exists() bool {
	return file.Exists(t.file())
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

func tmpBaseDir() string {
	return filepath.FromSlash("/dev/shm/opensvc")
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
