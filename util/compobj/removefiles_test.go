package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemovefileAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRule      string
		expectedRules []interface{}
	}{
		"with an empty rule": {
			jsonRule:      "[]",
			expectedRules: []interface{}{},
		},

		"with a rule with one file": {
			jsonRule:      `["totofile"]`,
			expectedRules: []interface{}{CompRemovefile("totofile")},
		},

		"with a rule with multiples files": {
			jsonRule:      `["totofile1", "totofile2", "totofile3", "totofile4"]`,
			expectedRules: []interface{}{CompRemovefile("totofile1"), CompRemovefile("totofile2"), CompRemovefile("totofile3"), CompRemovefile("totofile4")},
		},
	}
	for name, c := range testCases {
		obj := CompRemovefiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
		t.Run(name, func(t *testing.T) {
			require.NoError(t, obj.Add(c.jsonRule))
			require.Equal(t, c.expectedRules, obj.rules)
		})
	}
}

func TestRemovefileCheckRuleAndFixRule(t *testing.T) {
	getPresentFilePath := func(rule CompRemovefile) CompRemovefile {
		tmpFilePath := filepath.Join(t.TempDir(), string(rule))
		_, err := os.Create(tmpFilePath)
		require.NoError(t, err)
		return CompRemovefile(tmpFilePath)
	}

	getNotPresentFilePath := func(rule CompRemovefile) CompRemovefile {
		tmpFilePath := filepath.Join(t.TempDir(), string(rule))
		return CompRemovefile(tmpFilePath)
	}

	testCases := map[string]struct {
		rule                CompRemovefile
		expectedCheckOutput ExitCode
	}{
		"with a false rule (file exist)": {
			rule:                getPresentFilePath("lala"),
			expectedCheckOutput: ExitNok,
		},

		"with a true rule (file does not exist)": {
			rule:                getNotPresentFilePath("lili"),
			expectedCheckOutput: ExitOk,
		},
	}

	obj := CompRemovefiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, c.expectedCheckOutput, obj.checkRule(c.rule))
			if c.expectedCheckOutput == ExitNok {
				require.Equal(t, ExitOk, obj.fixRule(c.rule))
				require.Equal(t, ExitOk, obj.checkRule(c.rule))
			}
			_, err := os.Stat(string(c.rule))
			require.Equal(t, true, os.IsNotExist(err))
		})
	}
}
