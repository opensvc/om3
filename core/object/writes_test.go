package object

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
)

// The data of an instance is written by its app, container and share
// resources: once they are stopped, it does not change. A standby resource,
// up on every node, and a disabled one are not counted.
func TestIsWriting(t *testing.T) {
	with := func(m map[string]resource.Status) instance.Status {
		return instance.Status{Resources: m}
	}
	assert.False(t, IsWriting(with(map[string]resource.Status{
		"fs#1":   {Status: status.Up},
		"sync#1": {Status: status.Up},
		"app#1":  {Status: status.Down},
	})), "only an fs and a sync up")
	assert.True(t, IsWriting(with(map[string]resource.Status{"app#1": {Status: status.Up}})))
	assert.True(t, IsWriting(with(map[string]resource.Status{"container#1": {Status: status.Warn}})))
	assert.True(t, IsWriting(with(map[string]resource.Status{"share#1": {Status: status.Up}})))
	assert.True(t, IsWriting(with(map[string]resource.Status{"app#1": {Status: status.NotApplicable}})), "an app saying nothing may write")
	assert.False(t, IsWriting(with(map[string]resource.Status{"app#1": {Status: status.StandbyUp, IsStandby: true}})), "a standby app runs on every node")
	assert.False(t, IsWriting(with(map[string]resource.Status{"app#1": {Status: status.Up, IsDisabled: true}})))
}

func writeRecord(t *testing.T, dir, node, state string, at time.Time) {
	t.Helper()
	filename := writesFile(dir, node)
	assert.NoError(t, os.WriteFile(filename, []byte(state+"\n"), 0o644))
	assert.NoError(t, os.Chtimes(filename, at, at))
}

// The data is written while one instance writes it, and stopped since the
// latest stop when none does: a node back after it died records its stop
// at its return, later than the start of the node that took over.
func TestWritesStoppedAt(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	assert.True(t, WritesStoppedAt(dir).IsZero(), "no record")

	writeRecord(t, dir, "n1", writesStopped, t0)
	assert.True(t, WritesStoppedAt(dir).Equal(t0))

	writeRecord(t, dir, "n2", writesRunning, t0.Add(time.Minute))
	writeRecord(t, dir, "n1", writesStopped, t0.Add(time.Hour))
	assert.True(t, WritesStoppedAt(dir).IsZero(), "n2 writes, whatever n1 recorded later")

	writeRecord(t, dir, "n2", writesStopped, t0.Add(2*time.Minute))
	assert.True(t, WritesStoppedAt(dir).Equal(t0.Add(time.Hour)), "the latest stop")
}

// A node the cluster lost wrote nothing since: its record says so, from the
// time it was lost, and the record of a node that was not writing is left.
func TestPeerWritesStopped(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	changed, err := PeerWritesStopped(dir, "n1", t0)
	assert.NoError(t, err)
	assert.False(t, changed, "no record")

	writeRecord(t, dir, "n1", writesRunning, t0.Add(-time.Hour))
	assert.True(t, WritesStoppedAt(dir).IsZero())
	changed, err = PeerWritesStopped(dir, "n1", t0)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, WritesStoppedAt(dir).Equal(t0), "stopped since it was lost")

	changed, err = PeerWritesStopped(dir, "n1", t0.Add(time.Minute))
	assert.NoError(t, err)
	assert.False(t, changed, "already stopped")
	assert.True(t, WritesStoppedAt(dir).Equal(t0))
}

// The status a peer publishes corrects its record: a node back after it was
// lost writes again, and a peer never seen writing is not recorded stopped.
func TestPeerWrites(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	up := instance.Status{UpdatedAt: t0, Resources: map[string]resource.Status{"app#1": {Status: status.Up}}}
	down := instance.Status{UpdatedAt: t0.Add(time.Minute), Resources: map[string]resource.Status{"app#1": {Status: status.Down}}}

	changed, err := PeerWrites(dir, "n1", down)
	assert.NoError(t, err)
	assert.False(t, changed, "never seen writing")

	writeRecord(t, dir, "n1", writesStopped, t0.Add(-time.Hour))
	changed, err = PeerWrites(dir, "n1", up)
	assert.NoError(t, err)
	assert.True(t, changed, "back, and writing")
	assert.True(t, WritesStoppedAt(dir).IsZero())

	changed, err = PeerWrites(dir, "n1", down)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, WritesStoppedAt(dir).Equal(t0.Add(time.Minute)), "stopped as of the status saying so")
}

// A node name with a path separator names no record: what it would read or
// write lies outside the var dir of the object.
func TestWritesNameRefusesPaths(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"", ".", "..", "../n1", "a/b", `a\b`} {
		_, err := setWrites(dir, name, true, time.Now())
		assert.Error(t, err, "%q", name)
	}
	changed, err := setWrites(dir, "n1", true, time.Now())
	assert.NoError(t, err)
	assert.True(t, changed)
}

// A node out of the scope of the object has its record forgotten: a running
// one would keep every copy aging.
func TestForgetWrites(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	writeRecord(t, dir, "n1", writesStopped, t0)
	writeRecord(t, dir, "n9", writesRunning, t0)
	assert.True(t, WritesStoppedAt(dir).IsZero())
	assert.NoError(t, ForgetWrites(dir, "n9"))
	assert.True(t, WritesStoppedAt(dir).Equal(t0))
	assert.NoError(t, ForgetWrites(dir, "n9"), "forgetting a record gone")
	assert.NoError(t, ForgetWrites(filepath.Join(dir, "absent"), "n1"), "no var dir")
}

// An object without a var dir, as one purged, has no record made for it.
func TestSetWritesCreatesNoVarDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "purged")
	changed, err := setWrites(dir, "n1", true, time.Now())
	assert.NoError(t, err)
	assert.False(t, changed)
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

// The local instance records its writes from its first status, creating the
// var dir that nothing else made yet.
func TestRecordWritesInCreatesTheLocalVarDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new")
	changed, err := recordWritesIn(dir, "n1", true, time.Now(), true)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, WritesStoppedAt(dir).IsZero(), "running")
}

// A status older than the record changes nothing: a peer lost is not made
// writing again by a status of it cached from before.
func TestSetWritesIgnoresOlderStatuses(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	writeRecord(t, dir, "n1", writesRunning, t0)
	changed, err := PeerWritesStopped(dir, "n1", t0.Add(time.Minute))
	assert.NoError(t, err)
	assert.True(t, changed)
	changed, err = setWrites(dir, "n1", true, t0.Add(30*time.Second))
	assert.NoError(t, err)
	assert.False(t, changed, "a status from before the loss")
	assert.True(t, WritesStoppedAt(dir).Equal(t0.Add(time.Minute)))
}

// A peer lost is recorded stopped even when the time of its last status,
// from its own clock, is ahead of the time it was lost, and a status of it
// from before the loss changes nothing after.
func TestPeerWritesStoppedWithAPeerClockAhead(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.Local)
	writeRecord(t, dir, "n1", writesRunning, t0.Add(time.Minute))
	changed, err := PeerWritesStopped(dir, "n1", t0)
	assert.NoError(t, err)
	assert.True(t, changed, "lost, whatever the clocks")
	stoppedAt := WritesStoppedAt(dir)
	assert.True(t, stoppedAt.After(t0.Add(time.Minute)), "stopped after its last status: %s", stoppedAt)
	changed, err = setWrites(dir, "n1", true, t0.Add(time.Minute))
	assert.NoError(t, err)
	assert.False(t, changed, "its last status replayed")
}

// A stop recorded in the future, after a peer clock ahead, is read as now at
// most: the writes missed do not count past the present.
func TestWritesStoppedAtIsNotInTheFuture(t *testing.T) {
	dir := t.TempDir()
	writeRecord(t, dir, "n1", writesStopped, time.Now().Add(time.Hour))
	assert.False(t, WritesStoppedAt(dir).After(time.Now()))
}
