package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type (
	fakeModule struct {
		checkCode   ExitCode
		fixCode     ExitCode
		fixableCode ExitCode
		info        ObjInfo
		add         error
		rules       []interface{}
	}
)

var (
	t I = &fakeModule{}
)

func (f *fakeModule) Fix() ExitCode        { return f.fixCode }
func (f *fakeModule) Check() ExitCode      { return f.checkCode }
func (f *fakeModule) Fixable() ExitCode    { return f.fixableCode }
func (f *fakeModule) Info() ObjInfo        { return f.info }
func (f *fakeModule) Add(s string) error   { return f.add }
func (f *fakeModule) Rules() []interface{} { return f.rules }

func Test_runAction(t *testing.T) {
	mOri := m
	defer func() {
		m = mOri
	}()
	m = map[string]func() interface{}{"fake": func() interface{} {
		return &fakeModule{
			checkCode:   ExitNok,
			fixCode:     ExitOk,
			fixableCode: ExitNotApplicable,
			info: ObjInfo{
				DefaultPrefix:  "foo",
				ExampleValue:   nil,
				Description:    "bar",
				FormDefinition: "",
			},
			add:   nil,
			rules: nil,
		}
	},
	}

	cases := map[string]struct {
		args        []string
		exitCode    ExitCode
		expectedOut string
		expectedErr string
	}{
		"badModule": {
			args:        []string{"badModule", "fix"},
			exitCode:    ExitNok,
			expectedErr: "badModule compliance object not found in the core collection\n",
		},
		"fix": {
			args:     []string{"fake", "fix"},
			exitCode: ExitOk,
		},
		"check": {
			args:     []string{"fake", "check"},
			exitCode: ExitNok,
		},
		"fixable": {
			args:     []string{"fake", "fixable"},
			exitCode: ExitNotApplicable,
		},
		"badAction": {
			args:        []string{"fake", "badAction"},
			exitCode:    ExitOk,
			expectedErr: "invalid action: badAction\n",
		},
	}

	for s, tc := range cases {
		t.Run(s, func(t *testing.T) {
			if s != "badModule" {
				execDir := t.TempDir()
				execName := filepath.Join(execDir, tc.args[0])
				require.NoError(t, os.Symlink(os.Args[0], filepath.Join(execDir, tc.args[0])))
				tc.args[0] = execName
			}
			var wOut, wErr io.ReadWriter
			wOut = os.Stdout
			wErr = os.Stderr
			var bErr, bOut []byte
			if tc.expectedOut != "" {
				wOut = bytes.NewBuffer(bOut)
			}
			if tc.expectedErr != "" {
				wErr = bytes.NewBuffer(bErr)
			}
			require.Equal(t, tc.exitCode, objMain(tc.args, wOut, wErr))

			if tc.expectedOut != "" {
				b := make([]byte, len(tc.expectedOut)+1000)
				i, _ := wOut.Read(b)
				require.Equal(t, tc.expectedOut, string(b[:i]))
			}
			if tc.expectedErr != "" {
				b := make([]byte, len(tc.expectedErr)+1000)
				i, _ := wErr.Read(b)
				require.Equal(t, tc.expectedErr, string(b[:i]))
			}
		})
	}
}
