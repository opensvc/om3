package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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
				path:  "tmp",
			}},
		},

		"with multiples rules": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"=","value":"test"}, {"key":"test2","op":"=","value":"test2"}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "=",
				Value: "test",
				path:  "tmp",
			}, CompKeyval{
				Key:   "test2",
				Op:    "=",
				Value: "test2",
				path:  "tmp",
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
				path:  "tmp",
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
				path:  "tmp",
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
				path:  "tmp",
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
				path:  "tmp",
			}},
		},

		"with value that is a list of string and op = IN": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":["2","4"]}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "IN",
				Value: []any{"2", "4"},
				path:  "tmp",
			}},
		},

		"with value that is a list of containing string and int and op = IN": {
			jsonRule:    `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":["2",4]}]}`,
			expectError: false,
			expectedRule: []any{CompKeyval{
				Key:   "test",
				Op:    "IN",
				Value: []any{"2", float64(4)},
				path:  "tmp",
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

		"with an empty list and op IN": {
			jsonRule:     `{"path" : "tmp","keys" :[{"key":"test","op":"IN","value":[]}]}`,
			expectError:  true,
			expectedRule: nil,
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
		numberOfSetRules int
		expectedResult   ExitCode
	}{
		"with a true rule and op =": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "yes",
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op =": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "bipbop",
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op >=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    ">=",
				Value: float64(21),
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op >=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    ">=",
				Value: float64(23),
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op <=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "<=",
				Value: float64(23),
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op <=": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "<=",
				Value: float64(20),
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op unset": {
			rule: CompKeyval{
				Key:   "zooo",
				Op:    "unset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 0,
			expectedResult:   ExitOk,
		},

		"with a false rule and op unset": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "unset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 0,
			expectedResult:   ExitNok,
		},

		"with a true rule and op IN": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "IN",
				Value: []any{float64(64), "zozzo", float64(22), "lili"},
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitOk,
		},

		"with a false rule and op IN": {
			rule: CompKeyval{
				Key:   "Port",
				Op:    "IN",
				Value: []any{float64(64), "zozzo", float64(28), "lili"},
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 1,
			expectedResult:   ExitNok,
		},

		"with a true rule and op reset": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 3,
			expectedResult:   ExitOk,
		},

		"with a false rule and op reset": {
			rule: CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			},
			numberOfSetRules: 0,
			expectedResult:   ExitNok,
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompKeyvals{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			keyValResetMap = map[string]int{} //reset the map
			if c.numberOfSetRules != -1 {
				keyValResetMap[c.rule.Key] = c.numberOfSetRules
			}
			var err error
			keyValFileFmtCache, err = os.ReadFile(c.rule.path) //set the cache
			keyValpath = c.rule.path
			require.NoError(t, err)
			obj.rules = append(obj.rules, c.rule)
			require.Equal(t, c.expectedResult, obj.Check())
		})
	}
}

func TestKeyvalFixRule(t *testing.T) {
	testCases := map[string]struct {
		rules    []interface{}
		resetMap map[string]int
	}{
		"with a true rule op =": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "no",
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a false rule op =": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "roro",
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a true rule op >=": {
			rules: []interface{}{CompKeyval{
				Key:   "Port",
				Op:    ">=",
				Value: float64(20),
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a false rule op >=": {
			rules: []interface{}{CompKeyval{
				Key:   "Port",
				Op:    ">=",
				Value: float64(30),
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a true rule op <=": {
			rules: []interface{}{CompKeyval{
				Key:   "Port",
				Op:    "<=",
				Value: float64(30),
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a false rule op <=": {
			rules: []interface{}{CompKeyval{
				Key:   "Port",
				Op:    "<=",
				Value: float64(20),
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a true rule op unset": {
			rules: []interface{}{CompKeyval{
				Key:   "l'usineRoumaine",
				Op:    "unset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a false rule op unset": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "unset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a true rule op IN": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "IN",
				Value: []any{float64(40), "no"},
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a false rule op IN": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "IN",
				Value: []any{float64(40), "lolo"},
				path:  "./testdata/keyval_golden",
			}},
			resetMap: nil,
		},

		"with a false rule and op reset, 0 set rules": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			}},
			resetMap: map[string]int{"UsePAM": 0},
		},

		"with a false rule and op reset, 2 set rules": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "=",
				Value: "toto",
				path:  "./testdata/keyval_golden",
			}, CompKeyval{
				Key:   "UsePAM",
				Op:    ">=",
				Value: float64(50),
				path:  "./testdata/keyval_golden",
			}, CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			}},
			resetMap: map[string]int{"UsePAM": 0},
		},

		"with a false rule and op reset, and one unset rule": {
			rules: []interface{}{CompKeyval{
				Key:   "UsePAM",
				Op:    "unset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			}, CompKeyval{
				Key:   "UsePAM",
				Op:    "reset",
				Value: nil,
				path:  "./testdata/keyval_golden",
			}},
			resetMap: map[string]int{"UsePAM": 0},
		},

		"with multiples reset rules": {
			rules: []interface{}{
				CompKeyval{
					Key:   "UsePAM",
					Op:    "reset",
					Value: nil,
					path:  "./testdata/keyval_golden",
				}, CompKeyval{
					Key:   "UsePAM",
					Op:    "=",
					Value: float64(30),
					path:  "./testdata/keyval_golden",
				}, CompKeyval{
					Key:   "UsePAM",
					Op:    "=",
					Value: float64(30),
					path:  "./testdata/keyval_golden",
				}, CompKeyval{
					Key:   "UsePAM",
					Op:    "reset",
					Value: nil,
					path:  "./testdata/keyval_golden",
				}},
			resetMap: map[string]int{"UsePAM": 0},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			oriFileContent, err := os.ReadFile(c.rules[0].(CompKeyval).path)
			require.NoError(t, err)
			newFile, err := os.Create(c.rules[0].(CompKeyval).path + "TmpCopy")
			for i, rule := range c.rules {
				ruleConv := rule.(CompKeyval)
				ruleConv.path += "TmpCopy"
				c.rules[i] = ruleConv
			}
			defer func() { require.NoError(t, os.Remove(c.rules[0].(CompKeyval).path)) }()
			_, err = newFile.Write([]byte(oriFileContent))
			require.NoError(t, err)
			require.NoError(t, newFile.Close())

			obj := CompKeyvals{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			obj.rules = c.rules
			keyValResetMap = c.resetMap
			require.Equal(t, ExitOk, obj.Fix())
			for key := range keyValResetMap {
				keyValResetMap[key] = 0
			}
			keyValpath = ""
			require.Equal(t, ExitOk, obj.Check())
		})
	}
}
