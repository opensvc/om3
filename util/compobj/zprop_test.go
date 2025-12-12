package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddZprop(t *testing.T) {
	testCases := map[string]struct {
		jsonRule     string
		expectError  bool
		expectedRule CompZprop
	}{
		"with a full rule": {
			jsonRule: `[{"name":"testname","prop":"testprop","op":"=","value":9}]`,
			expectedRule: CompZprop{
				Name:  "testname",
				Prop:  "testprop",
				Op:    "=",
				Value: float64(9),
			},
		},

		"with no name": {
			jsonRule:     `[{"prop":"testprop","op":"=","value":9}]`,
			expectedRule: CompZprop{},
			expectError:  true,
		},

		"with no prop": {
			jsonRule:     `[{"name":"testname","op":"=","value":9}]`,
			expectedRule: CompZprop{},
			expectError:  true,
		},

		"with no op": {
			jsonRule:     `[{"name":"testname","prop":"testprop","value":9}]`,
			expectedRule: CompZprop{},
			expectError:  true,
		},

		"with op not in =, >=, <=": {
			jsonRule:     `[{"name":"testname","prop":"testprop","op":">>","value":9}]`,
			expectedRule: CompZprop{},
			expectError:  true,
		},

		"with a value of type string and not =": {
			jsonRule:     `[{"name":"testname","prop":"testprop","op":">=","value":"toto"}]`,
			expectedRule: CompZprop{},
			expectError:  true,
		},

		"with no value": {
			jsonRule:     `[{"name":"testname","prop":"testprop","op":"="}]`,
			expectError:  true,
			expectedRule: CompZprop{},
		},

		"with a value that is not int or string": {
			jsonRule:     `[{"name":"testname","prop":"testprop","op":"=","value":["tot","r"]}]`,
			expectError:  true,
			expectedRule: CompZprop{},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompZprops{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.add(c.jsonRule))
			} else {
				require.NoError(t, obj.add(c.jsonRule))
				require.Equal(t, c.expectedRule, obj.Obj.rules[0].(CompZprop))
			}
		})
	}
}

func TestCheckZbin(t *testing.T) {
	oriZpropZbin := zpropZbin
	defer func() { zpropZbin = oriZpropZbin }()

	testCases := map[string]struct {
		zbin           string
		expectedOutput ExitCode
	}{
		"with binary that is in path": {
			zbin:           "pwd",
			expectedOutput: ExitOk,
		},

		"with binary that is not in path": {
			zbin:           "iamnotinpath",
			expectedOutput: ExitNok,
		},
	}
	obj := CompZprops{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			zpropZbin = c.zbin
			require.Equal(t, c.expectedOutput, obj.checkZbin())
		})
	}
}

func TestCheckOperator(t *testing.T) {
	oriTGetProp := tgetProp
	defer func() { tgetProp = oriTGetProp }()

	testCases := map[string]struct {
		rule               CompZprop
		getPropReturnValue string
		expectedOutput     ExitCode
	}{
		"with a true string value and op =": {
			rule: CompZprop{
				Name:  "test",
				Prop:  "testProp",
				Op:    "=",
				Value: "val",
			},
			getPropReturnValue: "val",
			expectedOutput:     ExitOk,
		},

		"with a false string value and op =": {
			rule: CompZprop{
				Name:  "test",
				Prop:  "testProp",
				Op:    "=",
				Value: "val",
			},
			getPropReturnValue: "false",
			expectedOutput:     ExitNok,
		},

		"with a true float64 value and op =": {
			rule: CompZprop{
				Name:  "test",
				Prop:  "testProp",
				Op:    "=",
				Value: float64(2),
			},
			getPropReturnValue: "2",
			expectedOutput:     ExitOk,
		},

		"with a false float64 value and op >=": {
			rule: CompZprop{
				Name:  "test",
				Prop:  "testProp",
				Op:    ">=",
				Value: float64(2),
			},
			getPropReturnValue: "1",
			expectedOutput:     ExitNok,
		},

		"with a false float64 value and op <=": {
			rule: CompZprop{
				Name:  "test",
				Prop:  "testProp",
				Op:    "<=",
				Value: float64(2),
			},
			getPropReturnValue: "89",
			expectedOutput:     ExitNok,
		},
	}

	obj := CompZprops{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			tgetProp = func(rule CompZprop) (string, error) {
				return c.getPropReturnValue, nil
			}

			require.Equal(t, c.expectedOutput, obj.checkOperator(c.rule))
		})
	}
}
