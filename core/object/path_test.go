package object

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/config"
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
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		path, err := NewPath(test.name, test.namespace, test.kind)
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
func TestNewPathFromString(t *testing.T) {
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
	}
	for input, test := range tests {
		t.Logf("%s", input)
		path, err := NewPathFromString(input)
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
	path, _ := NewPath("svc1", "ns1", "svc")
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
		var path Path
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
	}
	for testName, test := range tests {
		config.Load(map[string]string{
			"osvc_root_path": test.root,
		})
		t.Logf("%s", testName)
		path, _ := NewPath(test.name, test.namespace, test.kind)
		assert.Equal(t, test.cf, path.ConfigFile())
	}

}
