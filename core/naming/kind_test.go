package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKind(t *testing.T) {
	tests := map[string]struct {
		kind   string
		output string
		ok     bool
	}{
		"valid kind": {
			kind:   "svc",
			output: "svc",
		},
		"invalid kind": {
			kind:   "invalid",
			output: "",
		},
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		k := ParseKind(test.kind)
		output := k.String()
		assert.Equal(t, test.output, output)
	}

}
