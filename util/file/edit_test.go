package file

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func editEnv(t *testing.T, content string) (src string) {
	t.Helper()
	src = filepath.Join(t.TempDir(), "o.conf")
	require.NoError(t, os.WriteFile(src, []byte(content), 0644))
	return src
}

func validNoBad(dst string) error {
	b, err := os.ReadFile(dst)
	if err != nil {
		return err
	}
	if string(b) == "bad\n" {
		return errors.New("bad")
	}
	return nil
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// An editor given with its arguments edits the copy, which is installed.
func TestEditWithAnEditorAndItsArguments(t *testing.T) {
	src := editEnv(t, "sleep 1000\n")
	t.Setenv("EDITOR", "sed -i s/1000/1001/")
	require.NoError(t, Edit(src, EditModeNormal, validNoBad))
	assert.Equal(t, "sleep 1001\n", read(t, src))
	assert.False(t, Exists(EditCopy(src)))
}

// An editor failing with the copy unchanged leaves no copy, so the next edit
// is not refused as pending.
func TestEditByAFailingEditorLeavesNoCopy(t *testing.T) {
	src := editEnv(t, "a\n")
	t.Setenv("EDITOR", "/bin/false")
	require.Error(t, Edit(src, EditModeNormal, validNoBad))
	assert.False(t, Exists(EditCopy(src)))
	t.Setenv("EDITOR", "/bin/true")
	require.NoError(t, Edit(src, EditModeNormal, validNoBad))
}

// A copy identical to the installed file holds nothing to recover: it is not
// an edit pending.
func TestEditOfACopyIdenticalToTheInstalledFileIsNotPending(t *testing.T) {
	src := editEnv(t, "a\n")
	require.NoError(t, Copy(src, EditCopy(src)))
	t.Setenv("EDITOR", "sed -i s/a/b/")
	require.NoError(t, Edit(src, EditModeNormal, validNoBad))
	assert.Equal(t, "b\n", read(t, src))
}

// Changes an editor made before failing are kept for a --recover, and a
// normal edit is refused while they are.
func TestEditByAFailingEditorKeepsItsChangesForARecover(t *testing.T) {
	src := editEnv(t, "a\n")
	t.Setenv("EDITOR", "sh -c 'echo c >\"$0\"; exit 1'")
	err := Edit(src, EditModeNormal, validNoBad)
	require.ErrorContains(t, err, "--recover")
	assert.Equal(t, "c\n", read(t, EditCopy(src)))
	t.Setenv("EDITOR", "/bin/true")
	require.ErrorIs(t, Edit(src, EditModeNormal, validNoBad), ErrEditPending)
	require.NoError(t, Edit(src, EditModeRecover, validNoBad))
	assert.Equal(t, "c\n", read(t, src))
	assert.False(t, Exists(EditCopy(src)))
}

// A recover whose editor changes nothing judges the copy against the
// installed file: an invalid copy is refused again and kept, rather than
// dropped as unchanged.
func TestARecoverChangingNothingKeepsAnInvalidCopy(t *testing.T) {
	src := editEnv(t, "a\n")
	require.NoError(t, os.WriteFile(EditCopy(src), []byte("bad\n"), 0644))
	t.Setenv("EDITOR", "/bin/true")
	require.ErrorIs(t, Edit(src, EditModeRecover, validNoBad), ErrEditValidate)
	assert.Equal(t, "bad\n", read(t, EditCopy(src)))
	assert.Equal(t, "a\n", read(t, src))
}
