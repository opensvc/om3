package resource

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// flagDriver is a driver whose provisioned flag lives in varDir.
type flagDriver struct {
	Driver
	varDir string
}

func (t *flagDriver) VarDir() string    { return t.varDir }
func (t *flagDriver) IsDisabled() bool  { return false }
func (t *flagDriver) Log() *plog.Logger { return plog.NewDefaultLogger() }
func (t *flagDriver) RID() string       { return "disk#1" }

func readFlag(t *testing.T, r Driver) bool {
	t.Helper()
	v, err := getProvisionedValue(r)
	require.NoError(t, err)
	return v
}

// A status evaluation found the resource up while its flag was false: the
// flag is set, found up, when it is still the flag read before the status
// was. An unprovision removing the resource and writing false while the
// status was read, the flag true before, is no flag the evaluation may answer
// for: it stays false, where it was turned back to true, and the unprovision
// orchestration waited for ever. Nor is a flag written again meanwhile.
func TestAFoundUpHoldsOnlyForAFlagUnchanged(t *testing.T) {
	ctx := context.Background()
	r := &flagDriver{varDir: t.TempDir()}

	require.NoError(t, setProvisionedValue(false, r))
	before := ReadProvisionedFlag(r)
	ok, err := SetProvisionedFoundSince(ctx, r, before)
	require.NoError(t, err)
	assert.True(t, ok, "a flag false and unchanged")
	assert.True(t, readFlag(t, r))

	// The race of the card: the flag true, the status reads the resource
	// up, the unprovision removes it and writes false, the status reads
	// false.
	before = ReadProvisionedFlag(r)
	require.NoError(t, setProvisionedValue(false, r))
	ok, err = SetProvisionedFoundSince(ctx, r, before)
	require.NoError(t, err)
	assert.False(t, ok, "a flag true before the evaluation")
	assert.False(t, readFlag(t, r), "the unprovision answer stays")

	before = ReadProvisionedFlag(r)
	require.NoError(t, setProvisionedValue(false, r))
	ok, _ = SetProvisionedFoundSince(ctx, r, before)
	assert.False(t, ok, "a flag written again during the evaluation")
	assert.False(t, readFlag(t, r))

	empty := &flagDriver{varDir: t.TempDir()}
	ok, _ = SetProvisionedFoundSince(ctx, empty, ReadProvisionedFlag(empty))
	assert.False(t, ok, "no flag read before the evaluation")
}

// The flag is written aside and renamed in place: a reader never finds the
// file empty or half written, which it took for no flag, and no temporary
// file is left.
func TestTheProvisionedFlagIsWrittenAtomically(t *testing.T) {
	r := &flagDriver{varDir: t.TempDir()}
	require.NoError(t, setProvisionedValue(true, r))

	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = setProvisionedValue(i%2 == 0, r)
		}
		close(done)
	}()
	for {
		select {
		case <-done:
			wg.Wait()
			entries, err := os.ReadDir(r.varDir)
			require.NoError(t, err)
			for _, e := range entries {
				assert.False(t, strings.HasPrefix(e.Name(), ".provisioned."), "temporary file left: %s", filepath.Join(r.varDir, e.Name()))
			}
			return
		default:
			_, err := getProvisionedValue(r)
			require.NoError(t, err, "a reader found the flag file half written")
		}
	}
}
