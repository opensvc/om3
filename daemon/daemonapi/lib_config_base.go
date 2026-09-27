package daemonapi

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/opensvc/om3/v3/core/naming"
)

// ErrConfigChanged is a configuration write refused because the file changed
// after the write was checked against it.
var ErrConfigChanged = errors.New("the configuration changed while the write was checked")

// configWriteMu serializes the check that the configuration file is still
// the one a write was checked against with the write itself, so no other
// write of the api lands between the two.
var configWriteMu sync.Mutex

// configBase is the configuration file a write was checked against.
//
// A write is checked, by the rbac policy, the validation and the claims,
// against the configuration read before it, and lands as a whole file. A
// file changed in between, as another user may change it, is replaced by
// the one checked against the older file: a keyword the other write changed
// is put back to the value it had, without the grant that change needs being
// asked of anyone. A cap holds its write until its orchestration is queued,
// which makes the window wide enough to hit. So the write lands only if the
// file is still the one it was checked against, and is refused otherwise,
// for the caller to ask again against the file as it now is.
//
// It is read before anything the checks read, so a file changed while they
// read it is one changed since the base, and is refused too.
type configBase struct {
	p      naming.Path
	data   []byte
	exists bool
}

func readConfigBase(p naming.Path) (configBase, error) {
	data, err := os.ReadFile(p.ConfigFile())
	switch {
	case errors.Is(err, os.ErrNotExist):
		return configBase{p: p}, nil
	case err != nil:
		return configBase{}, err
	}
	return configBase{p: p, data: data, exists: true}, nil
}

// commit runs the write if the configuration file is still the base.
func (t configBase) commit(write func() error) error {
	configWriteMu.Lock()
	defer configWriteMu.Unlock()
	current, err := readConfigBase(t.p)
	if err != nil {
		return err
	}
	if current.exists != t.exists || !bytes.Equal(current.data, t.data) {
		return fmt.Errorf("%w: the configuration of %s was written by someone else meanwhile, ask again", ErrConfigChanged, t.p)
	}
	return write()
}
