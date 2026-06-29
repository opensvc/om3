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

func TestParseRelations(t *testing.T) {
	// Test cases from the docstring examples with namespace "test"
	// Note: The docstring shows the expected Path representation (namespace/kind/name)
	// but Relation.String() for root namespace paths returns just kind/name (e.g., "svc2" instead of "root/svc/svc2")
	// So we verify the Path representation matches the docstring expectations
	tests := []struct {
		name           string
		input          []string
		ns             string
		expectedPaths  []Path
		expectedNodes  []string
	}{
		{
			name:  "explicitly local",
			input: []string{"./svc/svc2"},
			ns:    "test",
			expectedPaths: []Path{
				{Name: "svc2", Namespace: "test", Kind: KindSvc},
			},
			expectedNodes: []string{""},
		},
		{
			name:  "implicitly local",
			input: []string{"svc3"},
			ns:    "test",
			expectedPaths: []Path{
				{Name: "svc3", Namespace: "test", Kind: KindSvc},
			},
			expectedNodes: []string{""},
		},
		{
			name:  "explicitly foreign",
			input: []string{"root/svc/svc2"},
			ns:    "test",
			expectedPaths: []Path{
				{Name: "svc2", Namespace: NsRoot, Kind: KindSvc},
			},
			expectedNodes: []string{""},
		},
		{
			name:  "implicitly local with scope",
			input: []string{"svc4@n1"},
			ns:    "test",
			expectedPaths: []Path{
				{Name: "svc4", Namespace: "test", Kind: KindSvc},
			},
			expectedNodes: []string{"n1"},
		},
		{
			name:  "multiple relations",
			input: []string{"./svc/svc2", "svc3", "root/svc/svc2", "svc4@n1"},
			ns:    "test",
			expectedPaths: []Path{
				{Name: "svc2", Namespace: "test", Kind: KindSvc},
				{Name: "svc3", Namespace: "test", Kind: KindSvc},
				{Name: "svc2", Namespace: NsRoot, Kind: KindSvc},
				{Name: "svc4", Namespace: "test", Kind: KindSvc},
			},
			expectedNodes: []string{"", "", "", "n1"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ParseRelations(test.input, test.ns)
			assert.Equal(t, len(test.expectedPaths), len(result))
			for i, rel := range result {
				path, err := rel.Path()
				assert.NoError(t, err)
				assert.Equal(t, test.expectedPaths[i], path)
				assert.Equal(t, test.expectedNodes[i], rel.Node())
			}
		})
	}
}
