package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRelation(t *testing.T) {
	tests := map[string]struct {
		objectPath string
		node       string
		ok         bool
	}{
		"svc1": {
			objectPath: "svc1",
			node:       "",
			ok:         true,
		},
		"svc1@": {
			objectPath: "svc1",
			node:       "",
			ok:         true,
		},
		"svc1@n1": {
			objectPath: "svc1",
			node:       "n1",
			ok:         true,
		},
		"svc1@n1@n1": {
			objectPath: "svc1",
			node:       "n1@n1",
			ok:         true,
		},
	}
	for input, test := range tests {
		t.Logf("input: '%s'", input)
		objectPath, node, err := Relation(input).Split()
		switch test.ok {
		case true:
			assert.Nil(t, err)
		case false:
			assert.NotNil(t, err)
			continue
		}
		assert.Equal(t, test.objectPath, objectPath.String())
		assert.Equal(t, test.node, node)
	}
}
