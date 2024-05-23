package naming

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/testhelper"
)

func TestNewPath(t *testing.T) {
	tests := map[string]struct {
		name      string
		namespace string
		kind      string
		output    string
		ok        bool
	}{
		"fully qualified": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			output:    "ns1/svc/svc1",
			ok:        true,
		},
		"implicit kind svc": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "",
			output:    "ns1/svc/svc1",
			ok:        true,
		},
		"cannonicalization": {
			name:      "svc1",
			namespace: "root",
			kind:      "svc",
			output:    "svc1",
			ok:        true,
		},
		"lowerization": {
			name:      "SVC1",
			namespace: "ROOT",
			kind:      "SVC",
			output:    "",
			ok:        false,
		},
		"one char name is allowed": {
			name:      "a",
			namespace: "root",
			kind:      "svc",
			output:    "a",
			ok:        true,
		},
		"invalid kind": {
			name:      "svc1",
			namespace: "root",
			kind:      "unknown",
			output:    "",
			ok:        false,
		},
		"invalid name": {
			name:      "name#",
			namespace: "root",
			kind:      "svc",
			output:    "",
			ok:        false,
		},
		"invalid name (longer than 63 char)": {
			name:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			namespace: "root",
			kind:      "svc",
			output:    "",
			ok:        false,
		},
		"invalid namespace": {
			name:      "name",
			namespace: "root#",
			kind:      "svc",
			output:    "",
			ok:        false,
		},
		"empty name": {
			name:      "",
			namespace: "root",
			kind:      "svc",
			output:    "",
			ok:        false,
		},
		"forbidden name": {
			name:      "svc",
			namespace: "root",
			kind:      "svc",
			output:    "",
			ok:        false,
		},
		"cluster": {
			name:      "cluster",
			namespace: "root",
			kind:      "ccfg",
			output:    "cluster",
			ok:        true,
		},
		"zero value": {
			name:      "",
			namespace: "",
			kind:      "",
			output:    "",
			ok:        false,
		},
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		path, err := NewPathFromStrings(test.namespace, test.kind, test.name)
		if test.ok {
			if ok := assert.Nil(t, err); !ok {
				return
			}
		} else {
			if ok := assert.NotNil(t, err); !ok {
				return
			}
		}
		output := path.String()
		assert.Equal(t, test.output, output)
	}
}

func TestPathsLen(t *testing.T) {
	p1, _ := ParsePath("ns1/svc/n1")
	p2, _ := ParsePath("ns1/svc/n2")
	assert.Equal(t, 0, len(Paths{}))
	assert.Equal(t, 1, len(Paths{p1}))
	assert.Equal(t, 2, len(Paths{p1, p2}))
	var l Paths
	assert.Equal(t, 0, len(l))
}

func TestParsePath(t *testing.T) {
	tests := map[string]struct {
		name      string
		namespace string
		kind      string
		ok        bool
	}{
		"svc1": {
			name:      "svc1",
			namespace: "root",
			kind:      "svc",
			ok:        true,
		},
		"svc/svc1": {
			name:      "svc1",
			namespace: "root",
			kind:      "svc",
			ok:        true,
		},
		"ns1/svc/svc1": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			ok:        true,
		},
		"ns1/foo/name": {
			name:      "",
			namespace: "",
			kind:      "",
			ok:        false,
		},
		"ns1/svc/name#": {
			name:      "",
			namespace: "",
			kind:      "",
			ok:        false,
		},
		"ns1#/svc/name": {
			name:      "",
			namespace: "",
			kind:      "",
			ok:        false,
		},
		"ns1/svc/": {
			name:      "",
			namespace: "",
			kind:      "",
			ok:        false,
		},
		"ns1/": {
			name:      "namespace",
			namespace: "ns1",
			kind:      "nscfg",
			ok:        true,
		},
		"ns1/nscfg/": {
			name:      "namespace",
			namespace: "ns1",
			kind:      "nscfg",
			ok:        true,
		},
		"cluster": {
			name:      "cluster",
			namespace: "root",
			kind:      "ccfg",
			ok:        true,
		},
		"": {
			name:      "",
			namespace: "",
			kind:      "",
			ok:        false,
		},
	}
	for input, test := range tests {
		t.Logf("input: '%s'", input)
		path, err := ParsePath(input)
		switch test.ok {
		case true:
			assert.Nil(t, err)
		case false:
			assert.NotNil(t, err)
			continue
		}
		assert.Equal(t, test.name, path.Name)
		assert.Equal(t, test.namespace, path.Namespace)
		assert.Equal(t, test.kind, path.Kind.String())
	}

}

func TestPathMarshalJSON(t *testing.T) {
	path, _ := ParsePath("ns1/svc/svc1")
	b, err := json.Marshal(path)
	assert.Nil(t, err)
	assert.Equal(t, b, []byte(`"ns1/svc/svc1"`))
}

func TestPathUnmarshalJSON(t *testing.T) {
	tests := map[string]struct {
		name      string
		namespace string
		kind      string
		ok        bool
	}{
		`"ns1/svc/svc1"`: {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			ok:        true,
		},
		`{}`: {
			name:      "",
			namespace: "",
			kind:      "",
			ok:        false,
		},
	}
	for s, test := range tests {
		t.Logf("json unmarshal %s", s)
		b := []byte(s)
		var path Path
		err := json.Unmarshal(b, &path)
		switch test.ok {
		case true:
			assert.Nil(t, err)
		case false:
			assert.NotNil(t, err)
		}
		assert.Equal(t, test.namespace, path.Namespace)
		assert.Equal(t, test.name, path.Name)
		assert.Equal(t, test.kind, path.Kind.String())
	}
}

func TestPathMatch(t *testing.T) {
	tests := map[string]struct {
		name      string
		namespace string
		kind      string
		pattern   string
		match     bool
	}{
		"ns1/svc/svc1 matches */svc/*": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			pattern:   "*/svc/*",
			match:     true,
		},
		"vol/vol1 matches */*/v*": {
			name:      "vol1",
			namespace: "",
			kind:      "vol",
			pattern:   "*/*/v*",
			match:     true,
		},
		"vol/vol1 matches vol/v*": {
			name:      "vol1",
			namespace: "",
			kind:      "vol",
			pattern:   "vol/v*",
			match:     true,
		},
		"vol/vol1 does not match v*": {
			name:      "vol1",
			namespace: "",
			kind:      "vol",
			pattern:   "v*",
			match:     false,
		},
		"ns1/svc/svc1 does not match svc/*": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			pattern:   "svc/*",
			match:     false,
		},
		"ns1/svc/svc1 matches */*/svc*": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			pattern:   "*/*/svc*",
			match:     true,
		},
		"ns1/svc/svc1 matches *": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			pattern:   "*",
			match:     true,
		},
		"svc1 matches *": {
			name:      "svc1",
			namespace: "root",
			kind:      "svc",
			pattern:   "*",
			match:     true,
		},
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		path, _ := NewPathFromStrings(test.namespace, test.kind, test.name)
		assert.Equal(t, test.match, path.Match(test.pattern))
	}
}

func TestPathsMerge(t *testing.T) {
	l1 := Paths{
		Path{"s1", "ns1", KindSvc},
		Path{"s2", "ns2", KindSvc},
	}
	l2 := Paths{
		Path{"s2", "ns2", KindSvc},
		Path{"v1", "ns1", KindVol},
	}
	l1l2 := Paths{
		Path{"s1", "ns1", KindSvc},
		Path{"s2", "ns2", KindSvc},
		Path{"v1", "ns1", KindVol},
	}
	l2l1 := Paths{
		Path{"s2", "ns2", KindSvc},
		Path{"v1", "ns1", KindVol},
		Path{"s1", "ns1", KindSvc},
	}
	merged := l1.Merge(l2)
	assert.Equal(t, merged.String(), l1l2.String())
	merged = l2.Merge(l1)
	assert.Equal(t, merged.String(), l2l1.String())

}

func TestPathsFilter(t *testing.T) {
	l := Paths{
		Path{"s1", "ns1", KindSvc},
		Path{"s2", "ns2", KindSvc},
		Path{"v1", "ns1", KindVol},
	}
	tests := []struct {
		pattern  string
		expected Paths
	}{
		{
			"s*",
			Paths{
				Path{"s1", "ns1", KindSvc},
				Path{"s2", "ns2", KindSvc},
			},
		},
		{
			"*/vol/*",
			Paths{
				Path{"v1", "ns1", KindVol},
			},
		},
	}
	for _, test := range tests {
		filtered := l.Filter(test.pattern)
		assert.Equal(t, filtered.String(), test.expected.String())
	}
}

func TestConfigFile(t *testing.T) {
	tests := map[string]struct {
		name      string
		namespace string
		kind      string
		cf        string
		root      string
	}{
		"namespaced, package install": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			cf:        "/etc/opensvc/namespaces/ns1/svc/svc1.conf",
			root:      "",
		},
		"rooted svc, package install": {
			name:      "svc1",
			namespace: "",
			kind:      "svc",
			cf:        "/etc/opensvc/svc1.conf",
			root:      "",
		},
		"rooted cfg, package install": {
			name:      "cfg1",
			namespace: "",
			kind:      "cfg",
			cf:        "/etc/opensvc/cfg/cfg1.conf",
			root:      "",
		},
		"cluster cfg, package install": {
			name:      "cluster",
			namespace: "",
			kind:      "ccfg",
			cf:        "/etc/opensvc/cluster.conf",
			root:      "",
		},
		"namespaced, dev install": {
			name:      "svc1",
			namespace: "ns1",
			kind:      "svc",
			cf:        "/opt/opensvc/etc/namespaces/ns1/svc/svc1.conf",
			root:      "/opt/opensvc",
		},
		"rooted svc, dev install": {
			name:      "svc1",
			namespace: "",
			kind:      "svc",
			cf:        "/opt/opensvc/etc/svc1.conf",
			root:      "/opt/opensvc",
		},
		"rooted cfg, dev install": {
			name:      "cfg1",
			namespace: "",
			kind:      "cfg",
			cf:        "/opt/opensvc/etc/cfg/cfg1.conf",
			root:      "/opt/opensvc",
		},
		"cluster cfg, dev install": {
			name:      "cluster",
			namespace: "",
			kind:      "ccfg",
			cf:        "/opt/opensvc/etc/cluster.conf",
			root:      "/opt/opensvc",
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			_ = testhelper.SetupEnvWithoutCreateMandatoryDirectories(testhelper.Env{
				TestingT: t,
				Root:     test.root,
			})
			p, _ := NewPathFromStrings(test.namespace, test.kind, test.name)
			require.Equal(t, test.cf, p.ConfigFile())
		})
	}
}

func TestPathStringer(t *testing.T) {
	p := Path{}
	assert.Equal(t, "", p.String())
}

func TestPathEqual(t *testing.T) {
	p := Path{Name: "foo", Namespace: "ns1", Kind: KindSvc}

	assert.True(t, p.Equal(Path{Name: "foo", Namespace: "ns1", Kind: KindSvc}))

	assert.False(t, p.Equal(Path{Name: "foo", Namespace: "ns2", Kind: KindSvc}))
	assert.False(t, p.Equal(Path{Name: "foo", Namespace: "ns1", Kind: KindCfg}))
	assert.False(t, p.Equal(Path{Name: "bar", Namespace: "ns1", Kind: KindSvc}))
}
