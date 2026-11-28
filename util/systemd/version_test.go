package systemd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	for s, want := range map[string]int{
		"systemd 249 (249.11-0ubuntu3.23)\n+PAM +AUDIT +SELINUX\n": 249,
		"systemd 239 (239-82.el8_10.2)\n+PAM\n":                    239,
		"systemd 257 (257-13.el10)\n":                              257,
		"systemd 255.4-1ubuntu8.4\n":                               255,
		"systemd 256~rc3\n":                                        256,
		"systemd 219":                                              219,
	} {
		n, err := ParseVersion(s)
		require.NoError(t, err, "%q", s)
		assert.Equal(t, want, n, "%q", s)
	}
	for _, s := range []string{"", "\n", "systemd", "systemd unknown", "upstart 1.5", "249"} {
		_, err := ParseVersion(s)
		assert.Error(t, err, "%q", s)
	}
}
