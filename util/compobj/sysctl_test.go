package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSysctlAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRules     string
		expectedRules []any
		expectError   bool
	}{
		"add with 1 full rule ": {
			jsonRules: `[{"key": "k","index": 1,"op": ">=","value": 256}]`,
			expectedRules: []any{CompSysctl{
				Key:   "k",
				Index: pti(1),
				Op:    ">=",
				Value: float64(256),
			}},
		},

		"add with 2 full rules ": {
			jsonRules: `[{"key": "k","index": 1,"op": ">=","value": 256},{"key": "k2","index": 12,"op": ">=","value": 2562}]`,
			expectedRules: []any{CompSysctl{
				Key:   "k",
				Index: pti(1),
				Op:    ">=",
				Value: float64(256),
			}, CompSysctl{
				Key:   "k2",
				Index: pti(12),
				Op:    ">=",
				Value: float64(2562),
			},
			},
		},

		"add with missing key ": {
			jsonRules:   `[{"index": 1,"op": ">=","value": 256}]`,
			expectError: true,
		},
		"add with missing index ": {
			jsonRules:   `[{"key" : "k","op": ">=","value": 256}]`,
			expectError: true,
		},
		"add with missing op ": {
			jsonRules:   `[{"key" : "k","index" : 1,"value": 256}]`,
			expectError: true,
		},
		"add with missing value ": {
			jsonRules:   `[{"key" : "k","index" : 1,"op": ">="}]`,
			expectError: true,
		},
		"add with wrong op": {
			jsonRules:   `[{"key" : "k","index" : 1,"op": ">>","value": 256}}]`,
			expectError: true,
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRules))
			} else {
				require.NoError(t, obj.Add(c.jsonRules))
				require.Equal(t, c.expectedRules, obj.Obj.rules)
			}
		})
	}
}
