package main

import (
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
