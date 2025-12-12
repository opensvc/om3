package main

import (
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"
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
				Value: "5",
			}},
		},

		"add with two rules and a contradiction": {
			jsonRule:      `[{"key" : "test", "op" : "=", "value" : 5},{"key" : "test", "op" : "unset", "value" : 5}]`,
			expectError:   false,
			expectedRules: []any{},
		},

		"add with three rules and a contradiction": {
			jsonRule:    `[{"key" : "test", "op" : "=", "value" : 5},{"key" : "test", "op" : "unset", "value" : 5},{"key" : "test2", "op" : "unset", "value" : 5}]`,
			expectError: false,
			expectedRules: []any{CompNodeconf{
				Key:   "test2",
				Op:    "unset",
				Value: "5",
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
				Value: "5",
			}, CompNodeconf{
				Key:   "test2",
				Op:    ">=",
				Value: "3",
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
			ruleNodeConf = map[string]CompNodeconf{}
			blacklistedNodeConf = map[string]any{}
		})
	}
}
func TestNodeConfCheckRuleFixRule(t *testing.T) {
	testCases := map[string]struct {
		rule                CompNodeconf
		needRoot            bool
		keys                []string
		expectedCheckResult ExitCode
	}{
		"with a true = rule (string)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "=",
				Value: "0",
			},
			needRoot:            true,
			keys:                []string{`section.test="0"`},
			expectedCheckResult: ExitOk,
		},

		"with a false = rule (string)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "=",
				Value: "9",
			},
			needRoot:            true,
			keys:                []string{`section.test="0"`},
			expectedCheckResult: ExitNok,
		},

		"with a true = rule (int)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "=",
				Value: "0",
			},
			needRoot:            true,
			keys:                []string{`section.test=0`},
			expectedCheckResult: ExitOk,
		},

		"with a true <= rule (int)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "<=",
				Value: "0",
			},
			needRoot:            true,
			keys:                []string{`section.test="-2"`},
			expectedCheckResult: ExitOk,
		},

		"with a false <= rule (int)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "<=",
				Value: "0",
			},
			needRoot:            true,
			keys:                []string{`section.test="7"`},
			expectedCheckResult: ExitNok,
		},

		"with a false >= rule (int)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    ">=",
				Value: "0",
			},
			needRoot:            true,
			keys:                []string{`section.test="-4"`},
			expectedCheckResult: ExitNok,
		},

		"with a true >= rule (int)": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    ">=",
				Value: "0",
			},
			needRoot:            true,
			keys:                []string{`section.test="4"`},
			expectedCheckResult: ExitOk,
		},

		"with a false unset rule": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "unset",
				Value: nil,
			},
			needRoot:            true,
			keys:                []string{`section.test="-64"`},
			expectedCheckResult: ExitNok,
		},

		"with a true unset rule": {
			rule: CompNodeconf{
				Key:   "section.test",
				Op:    "unset",
				Value: nil,
			},
			needRoot:            true,
			keys:                []string{},
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
			o, err := object.NewNode()
			require.NoError(t, err)
			require.NoError(t, o.Config().Set(keyop.ParseList(c.keys...)...))
			require.Equal(t, c.expectedCheckResult, obj.checkRule(c.rule))
			require.Equal(t, ExitOk, obj.fixRule(c.rule))
			require.Equal(t, ExitOk, obj.checkRule(c.rule))
			o.Config().Unset(key.New("section", "test"))
		})
	}
}
