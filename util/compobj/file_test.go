package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/file"
)

func TestFile(t *testing.T) {
	type prepareEnv func(t *testing.T, file *CompFile)
	type testCase struct {
		envs        []prepareEnv
		rule        CompFile
		expectCheck ExitCode
		expectFix   ExitCode
		needRoot    bool
	}

	orig := collectorSafeGetMetaFunc
	defer func() {
		collectorSafeGetMetaFunc = orig
	}()
	collectorSafeGetMetaFunc = func(safePath string) (SafeFileMeta, error) {
		safePath = strings.Replace(safePath, "safe://", "", 1)
		md5, err := file.MD5(safePath)
		if err != nil {
			return SafeFileMeta{}, err
		}
		return SafeFileMeta{MD5: hex.EncodeToString(md5)}, nil
	}

	startServer := func(addr string) func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("a response\n"))
		})
		s := http.Server{Addr: addr, Handler: mux}
		go func() {
			t.Logf("starting server %s", addr)
			_ = s.ListenAndServe()
		}()
		return func() {
			_ = s.Shutdown(context.Background())
			t.Logf("shutdowned server %s", addr)
		}
	}

	defer startServer(":8080")()
	time.Sleep(time.Millisecond)
	t.Run("ensure fake web server is running", func(t *testing.T) {
		get, err := http.Get("http://localhost:8080/")
		require.NoError(t, err)
		b := make([]byte, 500)
		l, err := get.Body.Read(b)
		require.Greater(t, l, 0)
		require.Equal(t, "a response\n", string(b[:l]))
	})

	withEmptyFile := func(t *testing.T, r *CompFile) {
		t.Helper()
		f := filepath.Join(t.TempDir(), "withEmptyFile")
		r.Path = f
		t.Logf("with file %s", f)
		created, err := os.Create(f)
		require.NoErrorf(t, err, "can't create file for rule: %s", f)
		require.NoError(t, created.Close())
	}

	withFileContent := func(t *testing.T, r *CompFile) {
		t.Helper()
		f := filepath.Join(t.TempDir(), "withFileContent")
		r.Path = f
		b, err := r.Content()
		require.NoError(t, err)
		if !strings.HasSuffix(string(b), "\n") {
			b = append(b, byte('\n'))
		}
		t.Logf("with file %s contents: '%s'", f, b)
		require.Nil(t, os.WriteFile(f, b, 0666))
	}

	withBadPerms := func(t *testing.T, r *CompFile) {
		require.NoError(t, os.Chmod(r.Path, os.FileMode(*r.Mode)^os.ModeSticky))
	}

	withPerms := func(t *testing.T, r *CompFile) {
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

	withUID := func(t *testing.T, r *CompFile) {
		t.Helper()
		err := os.Chown(r.Path, r.UID.(int), -1)
		require.NoError(t, err)
		t.Logf("with uid %d for file: '%s'", r.UID.(int), r.Path)
	}

	withGid := func(t *testing.T, r *CompFile) {
		t.Helper()
		err := os.Chown(r.Path, -1, r.GID.(int))
		require.NoError(t, err)
		t.Logf("with gid %d for file: '%s'", r.GID.(int), r.Path)
	}

	withWrongUID := func(t *testing.T, r *CompFile) {
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

	withWrongGid := func(t *testing.T, r *CompFile) {
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

	withSafeRef := func(t *testing.T, r *CompFile) {
		t.Helper()
		withEmptyFile(t, r)
		r.Ref = "safe://" + r.Path
	}

	withWrongSafeRef := func(t *testing.T, r *CompFile) {
		t.Helper()
		withEmptyFile(t, r)
		r.Ref = "safe://" + r.Path
		withEmptyFile(t, r)
		err := os.WriteFile(r.Path, []byte("content"), 0666)
		require.NoError(t, err)
	}

	obj := CompFiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	pts := func(s string) *string { return &s }
	pti := func(i int) *int { return &i }
	cases := map[string]testCase{
		"with empty file": {
			envs:        []prepareEnv{withEmptyFile},
			rule:        CompFile{Fmt: pts("content\n")},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with correct content": {
			envs:        []prepareEnv{withFileContent},
			rule:        CompFile{Fmt: pts("content\n")},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with correct content and no carriage return and the end of the content": {
			envs:        []prepareEnv{withFileContent},
			rule:        CompFile{Fmt: pts("content")},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with correct content from ref": {
			envs:        []prepareEnv{withFileContent},
			rule:        CompFile{Ref: "http://localhost:8080/"},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with safe ref": {
			envs:        []prepareEnv{withSafeRef},
			rule:        CompFile{},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with wrong safe ref": {
			envs:        []prepareEnv{withWrongSafeRef},
			rule:        CompFile{},
			expectCheck: ExitNok,
			expectFix:   ExitNok},

		"with bad perms (file mode)": {
			envs:        []prepareEnv{withFileContent, withBadPerms},
			rule:        CompFile{Fmt: pts("content\n"), Mode: pti(666)},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with correct perms (file mode)": {
			envs:        []prepareEnv{withFileContent, withPerms},
			rule:        CompFile{Fmt: pts("content\n"), Mode: pti(666)},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with no field": {
			envs:        []prepareEnv{},
			rule:        CompFile{},
			expectCheck: ExitNok,
			expectFix:   ExitNok},

		"with uid (file mode)": {
			envs:        []prepareEnv{withEmptyFile, withUID},
			rule:        CompFile{UID: 1600},
			expectCheck: ExitOk,
			expectFix:   ExitOk,
			needRoot:    true},

		"with gid (file mode)": {
			envs:        []prepareEnv{withEmptyFile, withGid},
			rule:        CompFile{GID: 1600},
			expectCheck: ExitOk,
			expectFix:   ExitOk,
			needRoot:    true},

		"with wrong uid (file mode)": {
			envs:        []prepareEnv{withEmptyFile, withWrongUID},
			rule:        CompFile{UID: 1600},
			expectCheck: ExitNok,
			expectFix:   ExitOk,
			needRoot:    true},

		"with wrong gid (file mode)": {
			envs:        []prepareEnv{withEmptyFile, withWrongGid},
			rule:        CompFile{GID: 1600},
			expectCheck: ExitNok,
			expectFix:   ExitOk,
			needRoot:    true},

		"with bad path (file mode)": {
			envs:        []prepareEnv{},
			rule:        CompFile{Path: filepath.Join(t.TempDir(), "wrongpath")},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with path (dir mode)": {
			envs:        []prepareEnv{},
			rule:        CompFile{Path: filepath.Join(t.TempDir()) + string(filepath.Separator)},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with bad path (dir mode)": {
			envs:        []prepareEnv{},
			rule:        CompFile{Path: filepath.Join(t.TempDir(), "wrongDir") + string(filepath.Separator)},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with uid (dir mode)": {
			envs:        []prepareEnv{withUID},
			rule:        CompFile{Path: t.TempDir(), UID: 1600},
			expectCheck: ExitOk,
			expectFix:   ExitOk,
			needRoot:    true},

		"with perms (dir mode)": {
			envs:        []prepareEnv{withPerms},
			rule:        CompFile{Path: filepath.Join(t.TempDir()) + string(filepath.Separator), Mode: pti(666)},
			expectCheck: ExitOk,
			expectFix:   ExitOk},

		"with bad perms (dir mode)": {
			envs:        []prepareEnv{withBadPerms},
			rule:        CompFile{Path: filepath.Join(t.TempDir()) + string(filepath.Separator), Mode: pti(666)},
			expectCheck: ExitNok,
			expectFix:   ExitOk},

		"with fmt (dir mode)": {
			envs:        []prepareEnv{},
			rule:        CompFile{Path: filepath.Join(t.TempDir()) + string(filepath.Separator), Fmt: pts("content")},
			expectCheck: ExitNok,
			expectFix:   ExitNok},
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
			if c.rule.Fmt != nil && c.expectFix == ExitOk {
				t.Run("read file after fix ok to verify content is fmt", func(t *testing.T) {
					b, err := os.ReadFile(c.rule.Path)
					require.NoError(t, err)
					var expected string
					if c.rule.Fmt != nil {
						if strings.HasSuffix(*c.rule.Fmt, "\n") {
							expected = *c.rule.Fmt
						} else {
							expected = string(*c.rule.Fmt + "\n")
						}
					}
					require.Equal(t, expected, string(b))
				})
			}
		})
	}
}

func TestAddFile(t *testing.T) {
	pts := func(s string) *string { return &s }
	testCases := map[string]struct {
		jsonRule     string
		expectError  bool
		expectedRule CompFile
	}{
		"with a true rule and fmt": {
			jsonRule: `{"path":"/tmp/test","fmt":"content"}`,
			expectedRule: CompFile{
				Path: "/tmp/test",
				Mode: nil,
				UID:  nil,
				GID:  nil,
				Fmt:  pts("content"),
				Ref:  "",
			},
		},

		"with a true rule and ref": {
			jsonRule: `{"path":"/tmp/test","ref":"content"}`,
			expectedRule: CompFile{
				Path: "/tmp/test",
				Mode: nil,
				UID:  nil,
				GID:  nil,
				Fmt:  nil,
				Ref:  "content",
			},
		},

		"with no path": {
			jsonRule:     `{"fmt":"content"}`,
			expectedRule: CompFile{},
			expectError:  true,
		},

		"with no ref and no fmt": {
			jsonRule:     `{"path":"content"}`,
			expectedRule: CompFile{},
			expectError:  true,
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompFiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRule))
			} else {
				require.NoError(t, obj.Add(c.jsonRule))
				require.Equal(t, c.expectedRule, obj.Rules()[0].(CompFile))
			}
		})
	}
}
