package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestKeyValsAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRule     string
		expectError  bool
		expectedRule []any
	}{
		"with a full (one rule)": {
			jsonRule:    `{"path":"tmp", "keys" : [{"key":"test","op":"=","value":"test"}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "=",
				Value: "test",
			}},
		},

		"with multiples rules": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"=","value":"test"}, {"key":"test2","op":"=","value":"test2"}]}`,
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
			jsonRule:     `{"path" : "tmp","keys" : [{"key":"test","op":"=","value":"test"}, {"key":"test","op":"unset","value":"test2"}]}`,
			expectError:  false,
			expectedRule: []any{},
		},

		"with missing key": {
			jsonRule:     `{"path":"tmp", "keys" : [{"op":"=","value":"test"}]}`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with missing op": {
			jsonRule:    `{"path":"tmp", "keys" : [{"key":"test","value":"test"}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "=",
				Value: "test",
			}},
		},

		"with missing value and op != unset": {
			jsonRule:     `{"path":"tmp", "keys" : [{"key":"test","op":"="}]}`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with missing value and op = unset": {
			jsonRule:    `{"path":"tmp", "keys" : [{"key":"test","op":"unset"}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "unset",
				Value: nil,
			}},
		},

		"with a false op": {
			jsonRule:     `{"path" : "tmp","keys" :[{"key":"test","op":">>>>","value":"2"}]}`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with value that is an int": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"=","value":1}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "=",
				Value: float64(1),
			}},
		},

		"with value that is a bool": {
			jsonRule:     `{"path" : "tmp","keys" :[{"key":"test","op":"=","value":true}]}`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with value that is a list and op != IN": {
			jsonRule:     `{"path" : "tmp","keys" :[{"key":"test","op":"=","value":[2,4]}]}`,
			expectError:  true,
			expectedRule: []any{},
		},

		"with value that is a list of int and op = IN": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":[2,4]}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "IN",
				Value: []any{float64(2), float64(4)},
			}},
		},

		"with value that is a list of string and op = IN": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":["2","4"]}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "IN",
				Value: []any{"2", "4"},
			}},
		},

		"with value that is a list of containing string and int and op = IN": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":["2",4]}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "IN",
				Value: []any{"2", float64(4)},
			}},
		},

		"with value that is a list of containing string and int and bool and op = IN": {
			jsonRule:     `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":["2",4,true]}]}`,
			expectError:  true,
			expectedRule: []any{},
		},

		"use <= with a string": {
			jsonRule:     `{"path" : "tmp","keys" :[{"key":"test","op":"<=","value":"laal"}]}`,
			expectError:  true,
			expectedRule: []any{},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			keyvalValidityMap = map[string]string{}
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

func TestKeyValsCheckRule(t *testing.T) {
	testCases := map[string]struct {
		rule             CompKeyval
		filePath         string
		numberOfSetRules int
		expectedResult   ExitCode
	}{
		"with a true rule and op =": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "yes",
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op =": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "bipbop",
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op >=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    ">=",
				Value: float64(21),
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op >=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    ">=",
				Value: float64(23),
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op <=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "<=",
				Value: float64(23),
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op <=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "<=",
				Value: float64(20),
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op unset": {
			rule: CompKeyval{
				Key:   "zooo",
				Op:    "unset",
				Value: nil,
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 0,
			expectedResult:   ExitOk,
		},

		"with a false rule and op unset": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "unset",
				Value: nil,
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 0,
			expectedResult:   ExitNok,
		},

		"with a true rule and op IN": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "IN",
				Value: []any{float64(64), "zozzo", float64(22), "lili"},
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op IN": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "IN",
				Value: []any{float64(64), "zozzo", float64(28), "lili"},
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op reset": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 3,
			expectedResult:   ExitOk,
		},

		"with a false rule and op reset": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
			},
			filePath:         "./testdata/keyval_golden",
			numberOfSetRules: 0,
			expectedResult:   ExitNok,
		},
	}
	obj := CompKeyvals{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			keyValResetMap = map[string]int{} //reset the map
			if c.numberOfSetRules != -1 {
				keyValResetMap[c.rule.Key] = c.numberOfSetRules
			}
			var err error
			keyValFileFmtCache, err = os.ReadFile(c.filePath) //set the cache
			keyValpath = c.filePath
			require.NoError(t, err)
			require.Equal(t, c.expectedResult, obj.checkRule(c.rule))
		})
	}
}
