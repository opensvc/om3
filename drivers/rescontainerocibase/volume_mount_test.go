package rescontainerocibase

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A volume_mounts entry is <source>:<target>[:<options>]. A source in a
// resource of the object holds a ':' of its own, and spans two fields.
func TestParseVolumeMount(t *testing.T) {
	for _, tc := range []struct {
		s                   string
		source, target, opt string
	}{
		{"/srv/data:/data", "/srv/data", "/data", "rw"},
		{"/srv/data:/data:ro", "/srv/data", "/data", "ro"},
		{"web-cfg/etc/nginx:/etc/nginx", "web-cfg/etc/nginx", "/etc/nginx", "rw"},
		{"web-cfg/etc/nginx:/etc/nginx:ro,z", "web-cfg/etc/nginx", "/etc/nginx", "ro,z"},
		{"volume#1:/haproxy/certs:/certs", "volume#1:/haproxy/certs", "/certs", "rw"},
		{"volume#1:/haproxy/certs:/certs:ro", "volume#1:/haproxy/certs", "/certs", "ro"},
		{"/srv/a#b:/x", "/srv/a#b", "/x", "rw"},
	} {
		source, target, opt, err := parseVolumeMount(tc.s)
		require.NoError(t, err, tc.s)
		assert.Equal(t, []string{tc.source, tc.target, tc.opt}, []string{source, target, opt}, tc.s)
	}
	for _, s := range []string{"/srv/data", "volume#1:/haproxy/certs", "a:b:c:d"} {
		_, _, _, err := parseVolumeMount(s)
		assert.Error(t, err, s)
	}
}
