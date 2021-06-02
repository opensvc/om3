package xexec

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/util/limits"
	"testing"
)

func TestCommandFromLimits(t *testing.T) {
	cases := map[string]struct {
		s            string
		l            limits.T
		expectedArgs []string
	}{
		"command_with_no_limis": {
			"/bin/ls foo bar",
			limits.T{},
			[]string{"/bin/ls", "foo", "bar"},
		},
		"command_with_some_limits": {
			"/bin/ls foo bar",
			limits.T{LimitMemLock: 2000, LimitNoFile: 9},

			[]string{
				"/bin/sh",
				"-c",
				"ulimit -n 9 && ulimit -l 2 && /bin/ls foo bar",
			},
		},
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			cmd, err := CommandFromLimits(cases[name].l, cases[name].s)
			require.Nil(t, err)
			assert.Equal(t, cases[name].expectedArgs, cmd.Args)
		})
	}
}
