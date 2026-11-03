package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePathPatternRel(t *testing.T) {
	for s, want := range map[string]string{
		"./sec/haproxy-*": "ns1/sec/haproxy-*",
		"ns2/cfg/*.web":   "ns2/cfg/*.web",
		"sec/ca-*":        "sec/ca-*",
		"root/sec/*":      "sec/*",
	} {
		p, err := ParsePathPatternRel(s, "ns1")
		require.NoError(t, err, s)
		assert.Equal(t, want, p.String(), s)
	}
	for _, s := range []string{"./sec/haproxy", "./sec/*-*", "./sec/Haproxy-*", "./nope/a-*", "a-*", "a/b/c/d-*"} {
		_, err := ParsePathPatternRel(s, "ns1")
		assert.ErrorIs(t, err, ErrInvalid, s)
	}
}

func TestPathPatternMatch(t *testing.T) {
	p, err := ParsePathPatternRel("./sec/haproxy-*", "ns1")
	require.NoError(t, err)
	for name, want := range map[string]string{
		"haproxy-www.example.com": "www.example.com",
		"haproxy-a":               "a",
		"haproxy-":                "",
		"haproxy":                 "",
		"other-a":                 "",
	} {
		got, ok := p.Match(name)
		assert.Equal(t, want, got, name)
		assert.Equal(t, want != "", ok, name)
	}
	assert.True(t, p.MatchPath(Path{Namespace: "ns1", Kind: KindSec, Name: "haproxy-a"}))
	assert.False(t, p.MatchPath(Path{Namespace: "ns2", Kind: KindSec, Name: "haproxy-a"}))
	assert.False(t, p.MatchPath(Path{Namespace: "ns1", Kind: KindCfg, Name: "haproxy-a"}))
}

// The object selector of a listing of execs or orchestrations matches the
// paths of the records: a path, a pattern, several of them.
func TestMatchPatterns(t *testing.T) {
	for _, c := range []struct {
		patterns, path string
		want           bool
	}{
		{"", "ns1/svc/web", true},
		{"ns1/svc/web", "ns1/svc/web", true},
		{"ns1/svc/web", "ns1/svc/db", false},
		{"c32t2*", "c32t201web", true},
		{"c32t2*", "ns1/svc/c32t201web", false},
		{"ns1/svc/*", "ns1/svc/db", true},
		{"ns1/**", "ns1/vol/data", true},
		{"**", "ns2/cfg/x", true},
		{"ns1/svc/db,web", "web", true},
		{"ns1/svc/db,web", "ns1/svc/web", false},
	} {
		assert.Equal(t, c.want, MatchPatterns(c.patterns, c.path), "%q on %q", c.patterns, c.path)
	}
}
