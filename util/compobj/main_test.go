package main

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
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
			exitCode:    ExitOk,
			expectedErr: "The compliance objects in this collection must be called via a symlink.\nCollection content:\n  fake\n",
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
		"info": {
			args:        []string{"fake", "info"},
			exitCode:    ExitOk,
			expectedOut: "Description\n===========\n\n    bar\n\n\nExample rule\n============\n\n::\n\n    null\n\n\nForm definition\n===============\n\n::\n\n\n\n\n",
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
			require.Equal(t, tc.exitCode, mainArgs(tc.args, wOut, wErr))

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
