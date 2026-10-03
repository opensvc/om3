package collector

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

func newTestAction() Action {
	return Action{
		Path:      naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "s1"},
		Action:    "start",
		Argv:      []string{"s1", "instance", "start"},
		Origin:    "user",
		SessionID: uuid.New(),
		ExecID:    uuid.New(),
		PID:       1234,
		Begin:     time.Now().Truncate(time.Millisecond),
	}
}

func TestActionPendingDirWriteRead(t *testing.T) {
	dir := ActionPendingDir(filepath.Join(t.TempDir(), "action_pending"))
	a := newTestAction()
	key := a.Key()
	assert.Equal(t, a.ExecID.String()+".test.svc.s1", key)

	require.Equal(t, ActionPhaseBegin, a.Phase())
	require.NoError(t, dir.Write(a), "a missing directory is created")

	got, err := dir.Read(key, ActionPhaseBegin)
	require.NoError(t, err)
	assert.True(t, a.Begin.Equal(got.Begin))
	got.Begin = a.Begin
	assert.Equal(t, a, got)

	_, err = dir.Read(key, ActionPhaseEnd)
	assert.ErrorIs(t, err, os.ErrNotExist, "the end is not written yet")

	a.End = a.Begin.Add(time.Second)
	a.Status = "ok"
	require.Equal(t, ActionPhaseEnd, a.Phase())
	require.NoError(t, dir.Write(a))
	got, err = dir.Read(key, ActionPhaseEnd)
	require.NoError(t, err)
	assert.Equal(t, "ok", got.Status)
	assert.True(t, a.End.Equal(got.End))

	begin, err := dir.Read(key, ActionPhaseBegin)
	require.NoError(t, err)
	assert.True(t, begin.End.IsZero(), "the end leaves the begin file as it was")
}

func TestActionPendingDirListAndRemove(t *testing.T) {
	dir := ActionPendingDir(t.TempDir())

	l, err := dir.List()
	require.NoError(t, err)
	assert.Empty(t, l)

	running := newTestAction()
	ended := newTestAction()
	acknowledged := newTestAction()
	require.NoError(t, dir.Write(running))
	require.NoError(t, dir.Write(ended))
	ended.End = time.Now()
	ended.Status = "err"
	require.NoError(t, dir.Write(ended))
	require.NoError(t, dir.WriteUUID(acknowledged.Key(), "oc3-uuid"))

	// a write in progress, and an unrelated file, are not keys
	require.NoError(t, os.WriteFile(filepath.Join(string(dir), "."+running.Key()+".begin.json.123"), nil, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(string(dir), "README"), nil, 0600))

	l, err = dir.List()
	require.NoError(t, err)
	byKey := make(map[string]ActionPendingKey)
	for _, k := range l {
		byKey[k.Key] = k
	}
	require.Len(t, byKey, 3)
	assert.True(t, byKey[running.Key()].HasBegin)
	assert.False(t, byKey[running.Key()].HasEnd)
	assert.True(t, byKey[ended.Key()].HasBegin)
	assert.True(t, byKey[ended.Key()].HasEnd)
	assert.True(t, byKey[acknowledged.Key()].HasUUID)
	assert.False(t, byKey[acknowledged.Key()].ModTime.IsZero())

	s, err := dir.ReadUUID(acknowledged.Key())
	require.NoError(t, err)
	assert.Equal(t, "oc3-uuid", s)
	s, err = dir.ReadUUID(running.Key())
	require.NoError(t, err)
	assert.Empty(t, s, "a begin not acknowledged has no uuid")

	require.NoError(t, dir.RemoveBegin(ended.Key()))
	_, err = dir.Read(ended.Key(), ActionPhaseBegin)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = dir.Read(ended.Key(), ActionPhaseEnd)
	assert.NoError(t, err, "removing the begin leaves the end")

	require.NoError(t, dir.RemoveAll(ended.Key()))
	require.NoError(t, dir.RemoveAll(acknowledged.Key()))
	require.NoError(t, dir.RemoveAll(acknowledged.Key()), "removing twice is not an error")
	l, err = dir.List()
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, running.Key(), l[0].Key)
}

func TestActionPendingDirListMissingDir(t *testing.T) {
	l, err := ActionPendingDir(filepath.Join(t.TempDir(), "absent")).List()
	assert.NoError(t, err)
	assert.Empty(t, l)
}
