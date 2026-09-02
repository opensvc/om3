package object

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactSecrets(t *testing.T) {
	cases := map[string]struct {
		kind string
		conf string
		want string
	}{
		"a cluster secret": {
			kind: "ccfg",
			conf: "[cluster]\nname = c1\nsecret = 3aaf0dae\n",
			want: "[cluster]\nname = c1\nsecret = ********\n",
		},
		"every key of a sec data section": {
			kind: "sec",
			conf: "[DEFAULT]\nid = 1\n\n[data]\npassword = crypt:aGk=\npassword2 = crypt:aG8=\n",
			want: "[DEFAULT]\nid = 1\n\n[data]\npassword = ********\npassword2 = ********\n",
		},
		"nothing in a svc, which declares no secret keyword": {
			kind: "svc",
			conf: "[app#1]\ntype = forking\nstart = /bin/true\n",
			want: "[app#1]\ntype = forking\nstart = /bin/true\n",
		},

		// The scanner this replaced rebuilt every line from a regexp
		// submatch, so a line it parsed loosely came back changed.
		"the comments and the blank lines around it": {
			kind: "ccfg",
			conf: "# leading\n[cluster]\n\n; why\nsecret = old\n\n# trailing\n",
			want: "# leading\n[cluster]\n\n; why\nsecret = ********\n\n# trailing\n",
		},
		"the spacing of the redacted line": {
			kind: "ccfg",
			conf: "[cluster]\nsecret    =    old\n",
			want: "[cluster]\nsecret    =    ********\n",
		},
		"an inline comment on the redacted line": {
			kind: "ccfg",
			conf: "[cluster]\nsecret = old # rotate me\n",
			want: "[cluster]\nsecret = ******** # rotate me\n",
		},
		"a value holding the comment markers": {
			kind: "ccfg",
			conf: "[cluster]\nname = a#b;c\nsecret = old\n",
			want: "[cluster]\nname = a#b;c\nsecret = ********\n",
		},
		"a key delimited with a colon": {
			kind: "ccfg",
			conf: "[cluster]\nsecret:old\n",
			want: "[cluster]\nsecret:********\n",
		},
		"a scoped secret": {
			kind: "ccfg",
			conf: "[cluster]\nsecret@n1 = old\n",
			want: "[cluster]\nsecret@n1 = ********\n",
		},
		"a CRLF document": {
			kind: "ccfg",
			conf: "[cluster]\r\nname = c1\r\nsecret = old\r\n",
			want: "[cluster]\r\nname = c1\r\nsecret = ********\r\n",
		},
		"a document with no trailing newline": {
			kind: "ccfg",
			conf: "[cluster]\nsecret = old",
			want: "[cluster]\nsecret = ********",
		},
	}
	for title, c := range cases {
		t.Run(title, func(t *testing.T) {
			got, err := RedactSecrets([]byte(c.conf), c.kind)
			require.NoError(t, err)
			require.Equal(t, c.want, string(got))
		})
	}
}

// A key defined twice is normalised to its last definition, so the redacted
// value is the one kept and the dropped line can not leak the secret.
func TestRedactSecretsOfARedefinedKey(t *testing.T) {
	got, err := RedactSecrets([]byte("[cluster]\nsecret = first\nsecret = second\n"), "ccfg")
	require.NoError(t, err)
	require.NotContains(t, string(got), "first")
	require.NotContains(t, string(got), "second")
	require.Contains(t, string(got), "********")
}

// A section defined twice is merged, so a secret in either occurrence is
// redacted.
func TestRedactSecretsOfARedefinedSection(t *testing.T) {
	got, err := RedactSecrets([]byte("[cluster]\nname = c1\n[cluster]\nsecret = old\n"), "ccfg")
	require.NoError(t, err)
	require.NotContains(t, string(got), "old")
	require.Contains(t, string(got), "********")
}

func TestRedactSecretsRefusesAnUnparseableConfig(t *testing.T) {
	_, err := RedactSecrets([]byte("[cluster\nsecret = old\n"), "ccfg")
	require.Error(t, err)
}

// The node keyword store is not the core objects store, and an empty kind
// selects it.
func TestRedactSecretsOfANodeConfig(t *testing.T) {
	got, err := RedactSecrets([]byte("[node]\nuuid = abcd\nenv = PRD\n"), "")
	require.NoError(t, err)
	require.Equal(t, "[node]\nuuid = ********\nenv = PRD\n", string(got))
}

func TestRedactSecretsKeepsTheRestOfARealConfigIntact(t *testing.T) {
	conf := "[DEFAULT]\nid = 0d5b12a5\n#start_timeout = 6s\n\n[cluster]\nnodes = n1 n2\nsecret = s\n\n# [hb#3]\n# type = multicast\n"
	got, err := RedactSecrets([]byte(conf), "ccfg")
	require.NoError(t, err)
	require.Equal(t, strings.Replace(conf, "secret = s", "secret = ********", 1), string(got))
}
