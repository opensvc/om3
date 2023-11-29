package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMpathAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRules     string
		expectError   bool
		expectedRules []interface{}
	}{
		"with a full rule": {
			jsonRules:   `[{"key":"lala", "op":"=", "value" : "ok"}]`,
			expectError: false,
			expectedRules: []interface{}{CompMpath{
				Key:   "lala",
				Op:    "=",
				Value: "ok",
			}},
		},

		"with missing key": {
			jsonRules:     `[{"op":"=", "value" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with missing op": {
			jsonRules:     `[{"key":"lala", "value" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with missing value": {
			jsonRules:     `[{"op":"=", "key" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with wrong op": {
			jsonRules:     `[{"key":"lala", "op":">>>", "value" : "ok"}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"when value is a bool": {
			jsonRules:     `[{"key":"lala", "op":"=", "value" : true}]`,
			expectError:   true,
			expectedRules: nil,
		},

		"with string value and op >=": {
			jsonRules:     `[{"key":"lala", "op":">=", "value" : "true"}]`,
			expectError:   true,
			expectedRules: nil,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompMpaths{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRules))
			} else {
				require.NoError(t, obj.Add(c.jsonRules))
				require.Equal(t, c.expectedRules, obj.rules)
			}
		})
	}
}
