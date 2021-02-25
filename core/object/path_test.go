package object

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
