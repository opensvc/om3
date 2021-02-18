package listener

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender(t *testing.T) {
	tests := map[string]struct {
		addr   net.IP
		port   int
		output string
	}{
		"bracketing of ipv6": {
			addr:   net.ParseIP("::"),
			port:   1215,
			output: "[::]:1215",
		},
		"non-bracketing of ipv4": {
			addr:   net.ParseIP("0.0.0.0"),
			port:   1215,
			output: "0.0.0.0:1215",
		},
		"empty addr special case": {
			addr:   net.ParseIP(""),
			port:   1215,
			output: ":1215",
		},
	}
	for testName, test := range tests {
		t.Logf("Running test case %s", testName)
		output := Render(test.addr, test.port)
		assert.IsType(t, test.output, "")
		assert.Equal(t, test.output, output)
	}

}
