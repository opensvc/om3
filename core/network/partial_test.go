package network

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/plog"
)

// failingNet is a network whose setup fails with err.
type failingNet struct {
	T
	err error
}

func (t *failingNet) Setup() error { return t.err }

func (t *failingNet) CNIConfigData() (interface{}, error) {
	return map[string]string{"name": t.Name()}, nil
}

// logNoder is a noder giving the network a logger alone.
type logNoder struct{}

func (logNoder) MergedConfig() *xconfig.T { return nil }
func (logNoder) Log() *plog.Logger        { return plog.NewDefaultLogger() }
func (logNoder) Nodes() ([]string, error) { return nil, nil }

func newFailingNet(err error) *failingNet {
	nw := &failingNet{err: err}
	nw.SetNoder(logNoder{})
	nw.SetName("rb")
	nw.SetNetwork("10.100.0.0/16")
	return nw
}

// TestSetupNetworkWritesTheCNIOfAPartialFailure pins that a network whose
// setup failed in a part of it only, as the route to one peer, has its cni
// configuration written all the same, the objects of the node using it with
// the other peers, and that the failure is still returned. A setup failing
// whole writes none.
func TestSetupNetworkWritesTheCNIOfAPartialFailure(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rb.conf")

	err := setupNetwork(nil, newFailingNet(errors.New("whole")), dir)
	assert.ErrorContains(t, err, "whole")
	assert.NoFileExists(t, p)

	err = setupNetwork(nil, newFailingNet(&PartialError{Err: errors.New("node n2: unreachable")}), dir)
	assert.ErrorContains(t, err, "node n2: unreachable")
	_, serr := os.Stat(p)
	require.NoError(t, serr, "the cni configuration is written")
}
