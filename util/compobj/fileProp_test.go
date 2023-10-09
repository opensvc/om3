package main

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFileprop(t *testing.T) {
	type prepareEnv func(t *testing.T, file *CompFileProp)
	type testCase struct {
		envs        []prepareEnv
		rule        CompFileProp
		expectCheck ExitCode
		expectFix   ExitCode
		needRoot    bool
	}

	withFile := func(t *testing.T, r *CompFileProp) {
		t.Helper()
		f := filepath.Join(t.TempDir(), "withEmptyFile")
		r.Path = f
		t.Logf("with file %s", f)
		created, err := os.Create(f)
		require.NoErrorf(t, err, "can't create file for rule: %s", f)
		require.NoError(t, created.Close())
	}

	withBadPerms := func(t *testing.T, r *CompFileProp) {
		require.NoError(t, os.Chmod(r.Path, os.FileMode(*r.Mode)^os.ModeSticky))
	}

	withPerms := func(t *testing.T, r *CompFileProp) {
		t.Helper()
		s := fmt.Sprintf("0%d", *r.Mode)
		i, err := strconv.ParseInt(s, 8, 32)
		require.NoError(t, err)
		if strings.HasSuffix(r.Path, "/") {
			err = os.Chmod(r.Path, os.FileMode(i)|os.ModeDir)
		} else {
			err = os.Chmod(r.Path, os.FileMode(i))
		}
		require.NoError(t, err)
		t.Logf("with perms %s for file: '%s'", "0"+strconv.Itoa(int(i)), r.Path)
	}

	withUid := func(t *testing.T, r *CompFileProp) {
		t.Helper()
		err := os.Chown(r.Path, r.UID.(int), -1)
		require.NoError(t, err)
		t.Logf("with Uid %d for file: '%s'", r.UID.(int), r.Path)
	}

	withGid := func(t *testing.T, r *CompFileProp) {
		t.Helper()
		err := os.Chown(r.Path, -1, r.GID.(int))
		require.NoError(t, err)
		t.Logf("with Gid %d for file: '%s'", r.GID.(int), r.Path)
	}

	withWrongUid := func(t *testing.T, r *CompFileProp) {
		t.Helper()
		var err error
		if r.UID.(int) == 1500 {
			err = os.Chown(r.Path, 1501, -1)
			t.Logf("with uid %d for file: '%s'", 1501, r.Path)
		} else {
			err = os.Chown(r.Path, 1500, -1)
			t.Logf("with uid %d for file: '%s'", 1500, r.Path)
		}
		require.NoError(t, err)
	}

	withWrongGid := func(t *testing.T, r *CompFileProp) {
		t.Helper()
		var err error
		if r.GID.(int) == 1500 {
			err = os.Chown(r.Path, -1, 1501)
			t.Logf("with gid %d for file: '%s'", 1501, r.Path)
		} else {
			err = os.Chown(r.Path, -1, 1500)
			t.Logf("with gid %d for file: '%s'", 1500, r.Path)
		}
		require.NoError(t, err)
	}

	obj := CompFilesProps{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	pti := func(i int) *int { return &i }
	cases := map[string]testCase{
		"with empty file": {
			envs:        []prepareEnv{withFile},
			rule:        CompFileProp{},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with bad perms (file mode)": {
			envs:        []prepareEnv{withFile, withBadPerms},
			rule:        CompFileProp{Mode: pti(666)},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with correct perms (file mode)": {
			envs:        []prepareEnv{withFile, withPerms},
			rule:        CompFileProp{Mode: pti(666)},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with no field": {
			envs:        []prepareEnv{},
			rule:        CompFileProp{},
			expectCheck: ExitNok,
			expectFix:   ExitNok},

		"with uid (file mode)": {
			envs:        []prepareEnv{withFile, withUid},
			rule:        CompFileProp{UID: 1600},
			expectCheck: ExitOk,
			expectFix:   ExitOk,
			needRoot:    true},

		"with gid (file mode)": {
			envs:        []prepareEnv{withFile, withGid},
			rule:        CompFileProp{GID: 1600},
			expectCheck: ExitOk,
			expectFix:   ExitOk,
			needRoot:    true},

		"with wrong uid (file mode)": {
			envs:        []prepareEnv{withFile, withWrongUid},
			rule:        CompFileProp{UID: 1600},
			expectCheck: ExitNok,
			expectFix:   ExitOk,
			needRoot:    true},

		"with wrong gid (file mode)": {
			envs:        []prepareEnv{withFile, withWrongGid},
			rule:        CompFileProp{GID: 1600},
			expectCheck: ExitNok,
			expectFix:   ExitOk,
			needRoot:    true},

		"with bad path (file mode)": {
			envs:        []prepareEnv{},
			rule:        CompFileProp{Path: filepath.Join(t.TempDir(), "wrongpath")},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with path (dir mode)": {
			envs:        []prepareEnv{},
			rule:        CompFileProp{Path: filepath.Join(t.TempDir()) + string(filepath.Separator)},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with bad path (dir mode)": {
			envs:        []prepareEnv{},
			rule:        CompFileProp{Path: filepath.Join(t.TempDir(), "wrongDir") + string(filepath.Separator)},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with uid (dir mode)": {
			envs:        []prepareEnv{withUid},
			rule:        CompFileProp{Path: t.TempDir(), UID: 1600},
			expectCheck: ExitOk,
			expectFix:   ExitOk,
			needRoot:    true},

		"with perms (dir mode)": {
			envs:        []prepareEnv{withPerms},
			rule:        CompFileProp{Path: filepath.Join(t.TempDir()) + string(filepath.Separator), Mode: pti(666)},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with bad perms (dir mode)": {
			envs:        []prepareEnv{withBadPerms},
			rule:        CompFileProp{Path: filepath.Join(t.TempDir()) + string(filepath.Separator), Mode: pti(666)},
			expectCheck: ExitNok,
			expectFix:   ExitOk},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if c.needRoot {
				usr, err := user.Current()
				require.NoError(t, err)
				if usr.Username != "root" {
					t.Skip("need root")
				}
			}
			for _, f := range c.envs {
				f(t, &c.rule)
			}
			t.Run("Check", func(t *testing.T) {
				t.Logf("check : %d", obj.CheckRule(c.rule))
				require.Equal(t, c.expectCheck, obj.CheckRule(c.rule))
			})
			t.Run("Fix", func(t *testing.T) {
				require.Equal(t, c.expectFix, obj.FixRule(c.rule))
			})
			if c.expectCheck == ExitNok && c.expectFix == ExitOk {
				t.Run("Check after succeed Fix should succeed", func(t *testing.T) {
					require.Equal(t, ExitOk, obj.CheckRule(c.rule))
				})
			}
			if c.expectCheck == ExitNok && c.expectFix == ExitNok {
				t.Run("Check continue to fail after failed fix", func(t *testing.T) {
					require.Equal(t, ExitNok, obj.CheckRule(c.rule))
				})
			}
		})
	}
}
