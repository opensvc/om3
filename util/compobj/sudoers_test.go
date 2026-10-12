package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckSyntax(t *testing.T) {
	pts := func(s string) *string { return &s }
	testCases := map[string]struct {
		rule              CompFile
		expectCheckResult ExitCode
	}{
		"with a true syntax": {
			rule: CompFile{
				Path: "",
				Mode: nil,
				UID:  nil,
				GID:  nil,
				Fmt:  pts("test ALL=NOPASSWD: ALL"),
				Ref:  "",
			},
			expectCheckResult: ExitOk,
		},

		"with a false syntax": {
			rule: CompFile{
				Path: "",
				Mode: nil,
				UID:  nil,
				GID:  nil,
				Fmt:  pts("test ALL=NOPASALL"),
				Ref:  "",
			},
			expectCheckResult: ExitNok,
		},
	}

	obj := CompSudoerss{CompFiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.expectCheckResult, obj.checkSyntax(c.rule))
		})
	}
}

// The content is given to visudo, not to a shell: a quote in it ends no
// quoting, and runs nothing.
func TestCheckSyntaxRunsNoShellOnTheContent(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	content := "test ALL=NOPASSWD: ALL\n' ; touch " + marker + " ; echo '"
	obj := CompSudoerss{CompFiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}}
	obj.checkSyntax(CompFile{Fmt: &content})
	_, err := os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist)
}
