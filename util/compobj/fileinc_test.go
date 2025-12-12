package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFileincAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRule     string
		expectError  bool
		expectedRule CompFileinc
	}{
		"with a true rule (with check)": {
			jsonRule:    `{"path":"/tmp/foo","check":"regex","fmt":"lala","strict_fmt":false}`,
			expectError: false,
			expectedRule: CompFileinc{
				Path:      "/tmp/foo",
				Check:     "regex",
				Replace:   "",
				Fmt:       "lala",
				StrictFmt: false,
				Ref:       "",
			},
		},

		"with a true rule (with replace)": {
			jsonRule:    `{"path":"/tmp/foo","replace":"regex","fmt":"lala","strict_fmt":false}`,
			expectError: false,
			expectedRule: CompFileinc{
				Path:      "/tmp/foo",
				Check:     "",
				Replace:   "regex",
				Fmt:       "lala",
				StrictFmt: false,
				Ref:       "",
			},
		},

		"with a no check and no replace": {
			jsonRule:     `{"path":"/tmp/foo","fmt":"lala","strict_fmt":false,"ref":"thisisaref"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},

		"with no path": {
			jsonRule:     `{"check":"regex","fmt":"lala","strict_fmt":false,"ref":"thisisaref"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},

		"with check and replace": {
			jsonRule:     `{"path":"/tmp/foo","replace":"regex","fmt":"lala","strict_fmt":false,"ref":"thisisaref","check":"lala"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},

		"with fmt and ref": {
			jsonRule:     `{"path":"/tmp/foo","replace":"regex","fmt":"lala","strict_fmt":false,"ref":"thisisaref","fmt":"thisisfmt"}`,
			expectError:  true,
			expectedRule: CompFileinc{},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompFileincs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRule))
			} else {
				require.NoError(t, obj.Add(c.jsonRule))
				require.Equal(t, c.expectedRule, obj.rules[0].(CompFileinc))
			}
		})
	}
}

func TestFileincCheckRule(t *testing.T) {
	fd, err := os.Create("./testdata/tooLargeFile")
	defer func() {
		require.NoError(t, os.Remove("./testdata/tooLargeFile"))
	}()
	require.NoError(t, err)
	_, err = fd.Seek(MAXSZ, 0)
	require.NoError(t, err)
	_, err = fd.Write([]byte("\n0"))
	require.NoError(t, fd.Close())
	require.NoError(t, err)
	testCases := map[string]struct {
		rule           CompFileinc
		expectedResult ExitCode
	}{
		"with a true rule but a file that is too large (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/tooLargeFile",
				Check:     "0",
				Replace:   "",
				Fmt:       "0",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a path that does not exist": {
			rule: CompFileinc{
				Path:      "./testdata/lalaalalal",
				Check:     "0",
				Replace:   "",
				Fmt:       "0",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a true rule (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo+",
				Replace:   "",
				Fmt:       "i am the fooooo",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitOk,
		},

		"with a false rule because of multiples patterns (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     ".o+",
				Replace:   "",
				Fmt:       "i am the fooooo",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a false rule because fmt does not match regex in check (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     ".o+",
				Replace:   "",
				Fmt:       "i am the f",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a false rule because the pattern is not in the file (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "iamnotinfile",
				Replace:   "",
				Fmt:       "i am the iamnotinfile",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a true rule and using strict fmt (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo+",
				Replace:   "",
				Fmt:       "foo",
				StrictFmt: true,
				Ref:       "",
			},
			expectedResult: ExitOk,
		},

		"with a false because strict fmt is not respected (fmt and check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo+",
				Replace:   "",
				Fmt:       "fo",
				StrictFmt: true,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a true rule using ref instead of fmt": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo+",
				Replace:   "",
				Fmt:       "",
				StrictFmt: false,
				Ref:       "http://localhost:8080/",
			},
			expectedResult: ExitOk,
		},

		"with a false rule using ref instead of fmt": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo+",
				Replace:   "",
				Fmt:       "",
				StrictFmt: true,
				Ref:       "http://localhost:8080/",
			},
			expectedResult: ExitNok,
		},

		"with a true rule (fmt and replace)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "",
				Replace:   "fo+",
				Fmt:       "foo",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitOk,
		},

		"with a false rule because fmt is not correct(fmt and replace)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "",
				Replace:   "fo+",
				Fmt:       "fooo",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitNok,
		},

		"with a true rule because there is no pattern(fmt and replace)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "",
				Replace:   "foazaz+",
				Fmt:       "foo",
				StrictFmt: false,
				Ref:       "",
			},
			expectedResult: ExitOk,
		},
	}
	startServer := func(addr string) func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("fooo\n"))
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
		require.Equal(t, "fooo\n", string(b[:l]))
	})

	obj := CompFileincs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.expectedResult, obj.checkRule(c.rule))
		})
	}
}

func TestFileincFixRule(t *testing.T) {
	fd, err := os.Create("./testdata/tooLargeFile")
	defer func() {
		require.NoError(t, os.Remove("./testdata/tooLargeFile"))
	}()
	require.NoError(t, err)
	_, err = fd.Seek(MAXSZ, 0)
	require.NoError(t, err)
	_, err = fd.Write([]byte("\n0"))
	require.NoError(t, fd.Close())
	require.NoError(t, err)

	obj := CompFileincs{Obj: &Obj{rules: make([]interface{}, 0), verbose: false}}

	testCases := map[string]struct {
		rule                     CompFileinc
		expectedFixResult        ExitCode
		withFileThatDoesNotExist bool
	}{
		"with a too large file": {
			rule: CompFileinc{
				Path:      "./testdata/tooLargeFile",
				Check:     "0",
				Replace:   "",
				Fmt:       "0",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult: ExitNok,
		},
		"with a path that does not exist": {
			rule: CompFileinc{
				Path:      "./testdata/lalaalalal",
				Check:     "0",
				Replace:   "",
				Fmt:       "0",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult:        ExitNok,
			withFileThatDoesNotExist: true,
		},
		"with a true rule": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo",
				Replace:   "",
				Fmt:       "foo",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult:        ExitOk,
			withFileThatDoesNotExist: false,
		},
		"with a check with no pattern present in the file": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "lala",
				Replace:   "",
				Fmt:       "je suis lala",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult:        ExitOk,
			withFileThatDoesNotExist: false,
		},

		"with too many pattern present in the file (check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "many",
				Replace:   "",
				Fmt:       "je suis many",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult:        ExitOk,
			withFileThatDoesNotExist: false,
		},
		"with strict fmt (check)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "fo",
				Replace:   "",
				Fmt:       "fo",
				StrictFmt: true,
				Ref:       "",
			},
			expectedFixResult:        ExitOk,
			withFileThatDoesNotExist: false,
		},

		"with one pattern false (replace)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "",
				Replace:   "fo",
				Fmt:       "replacement",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult:        ExitOk,
			withFileThatDoesNotExist: false,
		},
		"with many patterns false in the same line (replace)": {
			rule: CompFileinc{
				Path:      "./testdata/fileinc_golden",
				Check:     "",
				Replace:   "o",
				Fmt:       "replacement",
				StrictFmt: false,
				Ref:       "",
			},
			expectedFixResult:        ExitOk,
			withFileThatDoesNotExist: false,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			if !c.withFileThatDoesNotExist {
				oriFileContent, err := os.ReadFile(c.rule.Path)
				require.NoError(t, err)
				newFile, err := os.Create(c.rule.Path + "TmpCopy")
				c.rule.Path = c.rule.Path + "TmpCopy"
				defer func() { require.NoError(t, os.Remove(c.rule.Path)) }()
				_, err = newFile.Write([]byte(oriFileContent))
				require.NoError(t, err)
				require.NoError(t, newFile.Close())
			}

			require.Equal(t, c.expectedFixResult, obj.fixRule(c.rule))
			if c.expectedFixResult == ExitOk {
				t.Run("check after fix", func(t *testing.T) {
					require.Equal(t, ExitOk, obj.checkRule(c.rule))
				})
			}
		})
	}
}
