package resfsflag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/file"
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	return &T{}
}

func (t T) Abort() bool {
	if len(t.Nodes) > 1 {
		// TODO
		return true
	}
	return false
}

// Start the Resource
func (t T) Start(ctx context.Context) error {
	if t.file() == "" {
		return errors.New("empty file path")
	}
	if t.exists() {
		t.Log().Info().Msgf("flag file %s is already installed", t.file())
		return nil
	}
	if err := os.MkdirAll(t.dir(), os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", t.dir())
	}
	t.Log().Info().Msgf("install flag file %s", t.file())
	if _, err := os.Create(t.file()); err != nil {
		return err
	}
	return nil
}

// Stop the Resource
func (t T) Stop(ctx context.Context) error {
	if t.file() == "" {
		return errors.New("empty file path")
	}
	if !t.exists() {
		t.Log().Info().Msgf("flag file %s is already uninstalled", t.file())
		return nil
	}
	t.Log().Info().Msgf("uninstall flag file %s", t.file())
	if err := os.Remove(t.file()); err != nil {
		return err
	}
	return nil
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return t.file()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	if t.file() == "" {
		t.StatusLog().Error("empty file path")
		return status.NotApplicable
	}
	if t.exists() {
		return status.Up
	}
	return status.Down
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) exists() bool {
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

func (t T) dir() string {
	var p string
	if t.lazyDir != "" {
		return t.lazyDir
	}
	if t.Path.Namespace != "root" {
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
