package main

import (
	"github.com/stretchr/testify/require"
	"testing"
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
