package main

import (
	"context"
	"github.com/opensvc/om3/core/object"
	"github.com/stretchr/testify/require"
	"os/user"
	"testing"
)

func TestNodeConfAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRule      string
		expectError   bool
		expectedRules []any
	}{
		"add with a true simple rule": {
			jsonRule:    `[{"key" : "test", "op" : "=", "value" : 5}]`,
			expectError: false,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},

		"add a rule with no key": {
			jsonRule:    `[{"op" : "=", "value" : 5}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},

		"add a rule with no op": {
			jsonRule:    `[{"key" : "test", "value" : 5}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},

		"add a rule with no value": {
			jsonRule:    `[{"key" : "test", "op" : "="}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},

		"add multiple rules": {
			jsonRule:    `[{"key" : "test", "op" : "=", "value" : 5}, {"key" : "test2", "op" : ">=", "value" : 3}]`,
			expectError: false,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}, CompNodeconf{
				Key:   "test2",
				Op:    ">=",
				Value: float64(3),
			}},
		},

		"with an operator that is not in =, <=, >=, unset": {
			jsonRule:    `[{"key" : "test", "op" : ">>", "value" : 5}]`,
			expectError: true,
			expectedRules: []any{CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			}},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompNodeconfs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			err := obj.Add(c.jsonRule)
			if c.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.expectedRules, obj.rules)
			}
		})
	}
}
func TestNodeConfCheckRule(t *testing.T) {
	oriOmGet := omGet
	defer func() { omGet = oriOmGet }()

	testCases := map[string]struct {
		rule                CompNodeconf
		needRoot            bool
		omGetOutput         string
		expectedCheckResult ExitCode
	}{
		"with a true unset rule": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    "unset",
				Value: "",
			},
			needRoot:            true,
			omGetOutput:         "",
			expectedCheckResult: ExitOk,
		},

		"with a false unset rule": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    "unset",
				Value: "",
			},
			needRoot:            true,
			omGetOutput:         "laala",
			expectedCheckResult: ExitNok,
		},

		"with a true = rule": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: "val",
			},
			needRoot:            true,
			omGetOutput:         "val",
			expectedCheckResult: ExitOk,
		},

		"with a false = rule": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: "valee",
			},
			needRoot:            true,
			omGetOutput:         "val",
			expectedCheckResult: ExitNok,
		},

		"a >= rule with a string": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    ">=",
				Value: "val",
			},
			needRoot:            true,
			omGetOutput:         "val",
			expectedCheckResult: ExitNok,
		},

		"a <= rule with a string": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    ">=",
				Value: "val",
			},
			needRoot:            true,
			omGetOutput:         "val",
			expectedCheckResult: ExitNok,
		},

		"a >= true rule with an int": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    ">=",
				Value: float64(3),
			},
			needRoot:            true,
			omGetOutput:         "5",
			expectedCheckResult: ExitOk,
		},

		"a >= false rule with an int": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    ">=",
				Value: float64(8),
			},
			needRoot:            true,
			omGetOutput:         "5",
			expectedCheckResult: ExitNok,
		},

		"a true = rule for an int": {
			rule: CompNodeconf{
				Key:   "test",
				Op:    "=",
				Value: float64(5),
			},
			needRoot:            true,
			omGetOutput:         "5",
			expectedCheckResult: ExitOk,
		},
	}

	obj := CompNodeconfs{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			if c.needRoot {
				usr, err := user.Current()
				require.NoError(t, err)
				if usr.Username != "root" {
					t.Skip("need root")
				}
			}
			omGet = func(node *object.Node, ctx context.Context, kw string) (interface{}, error) {
				return c.omGetOutput, nil
			}
			require.Equal(t, c.expectedCheckResult, obj.checkRule(c.rule))
		})
	}
}
