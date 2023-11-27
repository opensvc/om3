package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestKeyValsAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRule     string
		expectError  bool
		expectedRule []any
	}{
		"with a full (one rule)": {
			jsonRule:    `[{"key":"test","op":"=","value":"test"}]`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "=",
				Value: "test",
			}},
		},

		"with multiples rules": {
			jsonRule:    `[{"key":"test","op":"=","value":"test"}, {"key":"test2","op":"=","value":"test2"}]`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "=",
				Value: "test",
			}, CompKeyval{
				Key:   "test2",
				Op:    "=",
				Value: "test2",
			}},
		},

		"with conflicts in rules": {
			jsonRule:     `[{"key":"test","op":"=","value":"test"}, {"key":"test","op":"unset","value":"test2"}]`,
			expectError:  false,
			expectedRule: []any{},
		},

		"with missing key": {
			jsonRule:     `[{"op":"=","value":"test"}]`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with missing op": {
			jsonRule:     `[{"key":"test","value":"test"}]`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with missing value and op != unset": {
			jsonRule:     `[{"key":"test","op":"="}]`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with missing value and op = unset": {
			jsonRule:     `[{"key":"test","op":"unset"}]`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with a false op": {
			jsonRule:     `[{"key":"test","op":">>>>","value":"2"}]`,
			expectError:  true,
			expectedRule: []any{},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompKeyvals{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRule))
			} else {
				require.NoError(t, obj.Add(c.jsonRule))
				require.Equal(t, c.expectedRule, obj.rules)
			}
		})
	}
}
