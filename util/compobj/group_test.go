package main

import (
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupCheckFilesNsswitch(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	obj := CompGroups{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}

	TestCases := map[string]struct {
		nsswitchFile   string
		expectedOutput bool
	}{
		"with files": {
			nsswitchFile:   "./testdata/nsswitch.conf_true",
			expectedOutput: true,
		},
		"with multiples fields and files": {
			nsswitchFile:   "./testdata/nsswitch.conf_multipleFields",
			expectedOutput: true,
		},
		"with no files": {
			nsswitchFile:   "./testdata/nsswitch.conf_noFilesPasswd_noFilesShadow_noFilesGroup",
			expectedOutput: false,
		},
	}

	for name, c := range TestCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.nsswitchFile)
			}

			require.Equal(t, c.expectedOutput, obj.checkFileNsswitch())
		})
	}
}

func TestGroupCheckGroup(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	obj := CompGroups{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}

	TestCases := map[string]struct {
		rule           CompGroup
		groupFile      string
		expectedOutput ExitCode
	}{
		"with a group that is supposed to be present and is present and true gid": {
			rule: CompGroup{
				Group: "zozoGroup",
				Gid:   pti(1111),
			},
			groupFile:      "./testdata/group_group_file",
			expectedOutput: ExitOk,
		},

		"with a group that is supposed to be present and is present but wrong gid": {
			rule: CompGroup{
				Group: "zozoGroup",
				Gid:   pti(2222),
			},
			groupFile:      "./testdata/group_group_file",
			expectedOutput: ExitNok,
		},

		"with a group that is supposed to be present and is not present (but gid is present)": {
			rule: CompGroup{
				Group: "WrongGroup",
				Gid:   pti(1111),
			},
			groupFile:      "./testdata/group_group_file",
			expectedOutput: ExitNok,
		},

		"with a group that is not supposed to be present and is not present": {
			rule: CompGroup{
				Group: "-notPresent",
				Gid:   nil,
			},
			groupFile:      "./testdata/group_group_file",
			expectedOutput: ExitOk,
		},

		"with a group that is not supposed to be present and is present": {
			rule: CompGroup{
				Group: "-zozoGroup",
				Gid:   nil,
			},
			groupFile:      "./testdata/group_group_file",
			expectedOutput: ExitNok,
		},
	}

	for name, c := range TestCases {
		t.Run(name, func(t *testing.T) {
			var err error
			groupFileContent, err = os.ReadFile(c.groupFile)
			require.NoError(t, err)
			require.Equal(t, c.expectedOutput, obj.checkGroup(c.rule))
		})
	}
}

func TestGroupAdd(t *testing.T) {
	sliceToMap := func(groups any) map[string]CompGroup {
		m := make(map[string]CompGroup)
		switch l := groups.(type) {
		case []any:
			for _, group := range l {
				g := group.(CompGroup)
				m[g.Group] = g
			}
		case []CompGroup:
			for _, group := range l {
				m[group.Group] = group
			}
		}
		return m
	}

	groupAData := `"groupA" : {"gid" : 1000}`
	groupABisData := `"groupA" : {"gid" : 2000}`
	groupADelData := `"-groupA" : {}`

	groupACompGroup := CompGroup{
		Group: "groupA",
		Gid:   pti(1000),
	}

	groupADelCompGroup := CompGroup{
		Group: "-groupA",
		Gid:   nil,
	}

	groupBData := `"groupA" : {"gid" : 3000}`

	groupBCompGroup := CompGroup{
		Group: "groupA",
		Gid:   pti(3000),
	}

	groupCData := `"-groupC" : {}`

	groupCCompGroup := CompGroup{
		Group: "-groupC",
		Gid:   nil,
	}

	testCases := map[string]struct {
		jsonRules      []string
		expectAddError bool
		expectedRules  []CompGroup
	}{
		"with a simple rule": {
			jsonRules:     []string{"{" + groupAData + "}"},
			expectedRules: []CompGroup{groupACompGroup},
		},
		"with two rules in the same json": {
			jsonRules:     []string{"{" + groupAData + "," + groupBData + "}"},
			expectedRules: []CompGroup{groupACompGroup, groupBCompGroup},
		},
		"with two rules in two different json": {
			jsonRules:     []string{"{" + groupAData + "}", "{" + groupBData + "}"},
			expectedRules: []CompGroup{groupACompGroup, groupBCompGroup},
		},
		"with two rules with the same group name": {
			jsonRules:     []string{"{" + groupABisData + "}", "{" + groupAData + "}"},
			expectedRules: []CompGroup{groupACompGroup},
		},
		"with a delete rule": {
			jsonRules:     []string{"{" + groupCData + "}"},
			expectedRules: []CompGroup{groupCCompGroup},
		},

		"with two rules and the same the first rule is a del rule": {
			jsonRules:     []string{"{" + groupADelData + "}", "{" + groupAData + "}"},
			expectedRules: []CompGroup{groupACompGroup},
		},

		"with two rules and the second rule is a del rule": {
			jsonRules:     []string{"{" + groupAData + "}", "{" + groupADelData + "}"},
			expectedRules: []CompGroup{groupADelCompGroup},
		},

		"with empty json": {
			jsonRules:      []string{`{"" : {}}`},
			expectAddError: true,
			expectedRules:  nil,
		},

		"with missing name": {
			jsonRules:      []string{`{"" : {"gid" : 3000}}`},
			expectAddError: true,
			expectedRules:  nil,
		},

		"with missing gid": {
			jsonRules:      []string{`{"zozo" : {}}`},
			expectAddError: true,
			expectedRules:  nil,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompGroups{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			for _, json := range c.jsonRules {
				if c.expectAddError {
					require.Error(t, obj.Add(json))
				} else {
					require.NoError(t, obj.Add(json))
				}
			}
			require.Equal(t, true, reflect.DeepEqual(sliceToMap(c.expectedRules), sliceToMap(obj.rules)))
		})
	}
}

func TestGroupFixRule(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	oriExecGroupDel := execGroupDel
	defer func() { execGroupDel = oriExecGroupDel }()

	oriExecGroupAdd := execGroupAdd
	defer func() { execGroupAdd = oriExecGroupAdd }()

	oriExecChGroupGid := execChGroupGid
	defer func() { execChGroupGid = oriExecChGroupGid }()

	type cmdCode int
	var (
		noCmdAction    cmdCode = 0
		delCmdAction   cmdCode = 1
		addCmdAction   cmdCode = 2
		chGidCmdAction cmdCode = 3
	)

	testCases := map[string]struct {
		rule              CompGroup
		currentGroupFile  string
		goldenGroupFile   string
		currentCmdAction  cmdCode
		expectedCmdAction cmdCode
		expectedFixOutput ExitCode
	}{

		"with zozoGroup present and not supposed to be present": {
			rule: CompGroup{
				Group: "-zozoGroup",
				Gid:   pti(1111),
			},
			currentGroupFile:  "./testdata/group_group_file",
			goldenGroupFile:   "./testdata/group_group_file_noZozoGroup",
			currentCmdAction:  noCmdAction,
			expectedCmdAction: delCmdAction,
			expectedFixOutput: ExitOk,
		},

		"with zozoGroup not present and supposed to be present": {
			rule: CompGroup{
				Group: "zozoGroup",
				Gid:   pti(1111),
			},
			currentGroupFile:  "./testdata/group_group_file_noZozoGroup",
			goldenGroupFile:   "./testdata/group_group_file",
			currentCmdAction:  noCmdAction,
			expectedCmdAction: addCmdAction,
			expectedFixOutput: ExitOk,
		},

		"with zozoGroup present but not the right gid": {
			rule: CompGroup{
				Group: "zozoGroup",
				Gid:   pti(1111),
			},
			currentGroupFile:  "./testdata/group_group_file_wrong_gid",
			goldenGroupFile:   "./testdata/group_group_file",
			currentCmdAction:  noCmdAction,
			expectedCmdAction: chGidCmdAction,
			expectedFixOutput: ExitOk,
		},

		"try to delete a group in the blacklist": {
			rule: CompGroup{
				Group: "-daemon",
				Gid:   nil,
			},
			currentGroupFile:  "./testdata/group_group_file",
			goldenGroupFile:   "./testdata/group_group_file",
			currentCmdAction:  noCmdAction,
			expectedCmdAction: noCmdAction,
			expectedFixOutput: ExitNok,
		},
	}

	obj := CompGroups{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {

		t.Run(name, func(t *testing.T) {
			var err error
			execGroupDel = func(groupName string) *exec.Cmd {
				c.currentCmdAction = delCmdAction
				c.currentGroupFile = c.goldenGroupFile
				return exec.Command("pwd")
			}

			execGroupAdd = func(groupName string, gid int) *exec.Cmd {
				c.currentCmdAction = addCmdAction
				c.currentGroupFile = c.goldenGroupFile
				return exec.Command("pwd")
			}

			execChGroupGid = func(groupName string, newGid int) *exec.Cmd {
				c.currentCmdAction = chGidCmdAction
				c.currentGroupFile = c.goldenGroupFile
				return exec.Command("pwd")
			}

			groupFileContent, err = os.ReadFile(c.currentGroupFile)
			require.NoError(t, err)
			require.Equal(t, c.expectedFixOutput, obj.fixGroup(c.rule))
			require.Equal(t, c.goldenGroupFile, c.currentGroupFile)
			require.Equal(t, c.expectedCmdAction, c.currentCmdAction)
		})
	}
}
