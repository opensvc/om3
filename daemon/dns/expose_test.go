package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExpose(t *testing.T) {
	cases := map[string]Expose{
		// The form of the publish keyword.
		"80":            {FrontendPort: 80, BackendPort: 80, Network: "tcp"},
		"53/udp":        {FrontendPort: 53, BackendPort: 53, Network: "udp"},
		"8443:443":      {FrontendPort: 8443, BackendPort: 443, Network: "tcp"},
		"8443:443/tcp6": {FrontendPort: 8443, BackendPort: 443, Network: "tcp6"},
		"http:8080/tcp": {FrontendPort: 80, BackendPort: 8080, Network: "tcp"},

		// The former form, still read.
		"443/tcp:8443":  {FrontendPort: 8443, BackendPort: 443, Network: "tcp"},
		"53/udp:5353":   {FrontendPort: 5353, BackendPort: 53, Network: "udp"},
		"8080/tcp:http": {FrontendPort: 80, BackendPort: 8080, Network: "tcp"},
	}
	for s, want := range cases {
		t.Run(s, func(t *testing.T) {
			got, err := ParseExpose(s)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

// TestParseExposeReadsBothFormsAlike pins that a declaration rewritten from
// the former form to the publish one makes the same record.
func TestParseExposeReadsBothFormsAlike(t *testing.T) {
	for legacy, current := range map[string]string{
		"443/tcp:8443": "8443:443/tcp",
		"53/udp":       "53/udp",
		"80/tcp:80":    "80/tcp",
	} {
		a, err := ParseExpose(legacy)
		require.NoError(t, err)
		b, err := ParseExpose(current)
		require.NoError(t, err)
		assert.Equal(t, a, b, "%s and %s", legacy, current)
	}
}

func TestParseExposeRefuses(t *testing.T) {
	for _, s := range []string{
		"",
		"80/sctp",
		"x/tcp",
		"x",
		"80/tcp:nosuchservicename",
		"nosuchservicename:80",
		"10.0.0.5:8443:443",
		"80/tcp/udp",
	} {
		_, err := ParseExpose(s)
		assert.Error(t, err, s)
	}
}
