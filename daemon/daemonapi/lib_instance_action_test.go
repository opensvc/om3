package daemonapi

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecError(t *testing.T) {
	exitErr := errors.New("exit status 1")
	cases := map[string]struct {
		stderr string
		want   string
	}{
		"command line error": {
			stderr: "Error: w3: node w3: the collector can not wake it\n",
			want:   "w3: node w3: the collector can not wake it",
		},
		"several lines": {
			stderr: "Error: n2: no mac\n\n  n3: no mac  \n",
			want:   "n2: no mac; n3: no mac",
		},
		"nothing written": {
			stderr: "",
			want:   "exit status 1",
		},
		"blank lines only": {
			stderr: "\n  \n",
			want:   "exit status 1",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := execError(execResult{err: exitErr, stderr: []byte(c.stderr)})
			assert.EqualError(t, err, c.want)
		})
	}
}
