package resource

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusIPAddrs(t *testing.T) {
	for name, tc := range map[string]struct {
		info map[string]any
		want []string
	}{
		"none":      {nil, nil},
		"empty":     {map[string]any{"ipaddr": ""}, nil},
		"one":       {map[string]any{"ipaddr": "10.0.0.1"}, []string{"10.0.0.1"}},
		"net.IP":    {map[string]any{"ipaddr": net.ParseIP("10.0.0.1")}, []string{"10.0.0.1"}},
		"several":   {map[string]any{"ipaddr": "10.0.0.1", "ipaddrs": []string{"10.0.0.1", "fd00::1"}}, []string{"10.0.0.1", "fd00::1"}},
		"from json": {map[string]any{"ipaddr": "10.0.0.1", "ipaddrs": []any{"10.0.0.1", "fd00::1"}}, []string{"10.0.0.1", "fd00::1"}},
	} {
		assert.Equal(t, tc.want, Status{Info: tc.info}.IPAddrs(), name)
	}
}
