package path

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
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
			output:    "svc1",
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
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		path, err := New(test.name, test.namespace, test.kind)
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
func TestParse(t *testing.T) {
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
	}
	for input, test := range tests {
		t.Logf("%s", input)
		path, err := Parse(input)
		switch test.ok {
		case true:
			assert.Nil(t, err)
		case false:
			assert.NotNil(t, err)
		}
		assert.Equal(t, test.name, path.Name)
		assert.Equal(t, test.namespace, path.Namespace)
		assert.Equal(t, test.kind, path.Kind.String())
	}

}

func TestMarshalJSON(t *testing.T) {
	path, _ := New("svc1", "ns1", "svc")
	b, err := json.Marshal(path)
	assert.Nil(t, err)
	assert.Equal(t, b, []byte(`"ns1/svc/svc1"`))
}

func TestUnmarshalJSON(t *testing.T) {
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
		var path T
		err := json.Unmarshal(b, &path)
		switch test.ok {
		case true:
			assert.Nil(t, err)
		case false:
			assert.NotNil(t, err)
		}
		assert.Equal(t, path.Namespace, test.namespace)
		assert.Equal(t, path.Name, test.name)
		assert.Equal(t, path.Kind.String(), test.kind)
	}
}

func TestMatch(t *testing.T) {
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
		path, _ := New(test.name, test.namespace, test.kind)
		assert.Equal(t, test.match, path.Match(test.pattern))
	}
}
