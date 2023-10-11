package main

import (
	"bufio"
	"bytes"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestGroupMembershipAdd(t *testing.T) {
	sliceToMap := func(groups any) map[string]CompGroupMembership {
		m := make(map[string]CompGroupMembership)
		switch l := groups.(type) {
		case []any:
			for _, group := range l {
				g := group.(CompGroupMembership)
				m[g.Group] = g
			}
		case []CompGroupMembership:
			for _, group := range l {
				m[group.Group] = group
			}
		}
		return m
	}

	groupAData := `"groupA" : {"members" : ["zozo","toto","titi"]}`
	groupABisData := `"groupA" : {"members" : ["-zozobis","totobis"]}`

	groupADataComp := CompGroupMembership{
		Group:   "groupA",
		Members: []string{"zozo", "toto", "titi"},
	}

	groupBData := `"groupB" : {"members" : ["zozoB","totoB","titiB","tataB"]}`

	groupBDataComp := CompGroupMembership{
		Group:   "groupB",
		Members: []string{"zozoB", "totoB", "titiB", "tataB"},
	}

	testCases := map[string]struct {
		jsonRules      []string
		expectAddError bool
		expectedRules  []CompGroupMembership
	}{
		"with an empty rule": {
			jsonRules:      []string{`{"" : {}}`},
			expectAddError: true,
			expectedRules:  nil,
		},

		"with missing group": {
			jsonRules:      []string{`{"" : {"members": ["user1","user2"]}}`},
			expectAddError: true,
			expectedRules:  nil,
		},

		"with a simple rule": {
			jsonRules:      []string{"{" + groupAData + "}"},
			expectAddError: false,
			expectedRules:  []CompGroupMembership{groupADataComp},
		},

		"with two rules in the same json": {
			jsonRules:      []string{"{" + groupAData + "," + groupBData + "}"},
			expectAddError: false,
			expectedRules:  []CompGroupMembership{groupADataComp, groupBDataComp},
		},

		"with two rules in two different json": {
			jsonRules:      []string{"{" + groupAData + "}", "{" + groupBData + "}"},
			expectAddError: false,
			expectedRules:  []CompGroupMembership{groupADataComp, groupBDataComp},
		},

		"with overriding (in two different json)": {
			jsonRules:      []string{"{" + groupABisData + "}", "{" + groupBData + "}", "{" + groupAData + "}"},
			expectAddError: false,
			expectedRules:  []CompGroupMembership{groupADataComp, groupBDataComp},
		},

		"with no members field": {
			jsonRules:      []string{`{"noMembersGroup" : {}}`},
			expectAddError: false,
			expectedRules: []CompGroupMembership{{
				Group:   "noMembersGroup",
				Members: nil,
			}},
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompGroupsMemberships{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
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

func TestGroupMembershipCheckRule(t *testing.T) {
	oriExecGetent := execGetent
	defer func() { execGetent = oriExecGetent }()

	oriExecId := execId
	defer func() { execId = oriExecId }()

	getLine := func(FileContent []byte, name string) string {
		scanner := bufio.NewScanner(bytes.NewReader(FileContent))
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), ":")
			if line[0] == name {
				return scanner.Text()
			}
		}
		return ""
	}

	getGroupNameFromId := func(groupFileContent []byte, id string) string {
		scanner := bufio.NewScanner(bytes.NewReader(groupFileContent))
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), ":")
			if len(line) < 3 {
				continue
			}
			if line[2] == id {
				return line[0]
			}
		}
		return ""
	}

	getPrimaryGroupId := func(passwdFileContent []byte, userName string) string {
		scanner := bufio.NewScanner(bytes.NewReader(passwdFileContent))
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), ":")
			if len(line) < 4 {
				continue
			}
			if line[0] == userName {
				return line[3]
			}
		}
		return ""
	}

	testCases := map[string]struct {
		rule                CompGroupMembership
		passwdFile          string
		groupFile           string
		expectedCheckOutput ExitCode
	}{
		"with a group that is not present": {
			rule: CompGroupMembership{
				Group:   "iAmNotPresent",
				Members: nil,
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitOk,
		},

		"with a group that is present but no members in the rule (nil)": {
			rule: CompGroupMembership{
				Group:   "fax",
				Members: nil,
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitOk,
		},

		"with members that needs to be present and are present as secondary group": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"user1", "user2"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitOk,
		},

		"with members that needs to be present and are present as secondary group and primary group": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"user1", "zozo", "user2"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitOk,
		},

		"with members that are not supposed to be present and are not present": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user3", "-user4"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitOk,
		},

		"with members that are supposed to be present and some are not present": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"user3", "user2"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitNok,
		},

		"with members that are not supposed to be present and some are present": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user3", "-user2"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitNok,
		},

		"with members that are not supposed to be present and one has the group as primary group": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user3", "-user2", "-zozo"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitNok,
		},

		"with a true rule : some users needs to be present some not on as the group as primary": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user3", "zozo", "user2"},
			},
			passwdFile:          "./testdata/groupMembership_passwd_file",
			groupFile:           "./testdata/groupMembership_group_file",
			expectedCheckOutput: ExitOk,
		},
	}
	obj := CompGroupsMemberships{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			execGetent = func(file string, name string) *exec.Cmd {
				var line string
				switch file {
				case "group":
					fileContent, err := os.ReadFile(c.groupFile)
					require.NoError(t, err)
					line = getLine(fileContent, name)
				case "passwd":
					fileContent, err := os.ReadFile(c.passwdFile)
					require.NoError(t, err)
					line = getLine(fileContent, name)
				default:
					line = `the param file in execGetent only takes as argument "group"" or "file" you should not see this line ! this line come from groupMembership_test.go`
				}
				return exec.Command("echo", "-n", line)
			}
			execId = func(userName string) *exec.Cmd {
				passwdFileContent, err := os.ReadFile(c.passwdFile)
				require.NoError(t, err)
				groupFileContent, err := os.ReadFile(c.groupFile)
				require.NoError(t, err)
				id := getPrimaryGroupId(passwdFileContent, userName)
				return exec.Command("echo", "-n", getGroupNameFromId(groupFileContent, id))
			}

			require.Equal(t, c.expectedCheckOutput, obj.checkRule(c.rule))
		})
	}
}

func TestGroupMembershipFixRule(t *testing.T) {
	oriExecGetent := execGetent
	defer func() { execGetent = oriExecGetent }()

	oriExecId := execId
	defer func() { execId = oriExecId }()

	oriExecUsermodAdd := execUsermodAdd
	defer func() { execUsermodAdd = oriExecUsermodAdd }()

	oriExecGpasswdDel := execGpasswdDel
	defer func() { execGpasswdDel = oriExecGpasswdDel }()

	type fixAction int

	var (
		addAction fixAction = 1
		delAction fixAction = 2
	)

	getLine := func(FileContent []byte, name string) string {
		scanner := bufio.NewScanner(bytes.NewReader(FileContent))
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), ":")
			if line[0] == name {
				return scanner.Text()
			}
		}
		return ""
	}

	getGroupNameFromId := func(groupFileContent []byte, id string) string {
		scanner := bufio.NewScanner(bytes.NewReader(groupFileContent))
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), ":")
			if len(line) < 3 {
				continue
			}
			if line[2] == id {
				return line[0]
			}
		}
		return ""
	}

	getPrimaryGroupId := func(passwdFileContent []byte, userName string) string {
		scanner := bufio.NewScanner(bytes.NewReader(passwdFileContent))
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), ":")
			if len(line) < 4 {
				continue
			}
			if line[0] == userName {
				return line[3]
			}
		}
		return ""
	}

	testCases := map[string]struct {
		rule                        CompGroupMembership
		passwdFile                  string
		groupFile                   string
		expectedFixActionsOnMembers map[string]fixAction
		expectedFixOutput           ExitCode
	}{
		"with a true rule": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"user1", "user2", "-user4"},
			},
			passwdFile:                  "./testdata/groupMembership_passwd_file",
			groupFile:                   "./testdata/groupMembership_group_file",
			expectedFixActionsOnMembers: map[string]fixAction{},
			expectedFixOutput:           ExitOk,
		},

		"with missing users (add)": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"user1", "user3", "user2", "user4", "zozo"},
			},
			passwdFile:                  "./testdata/groupMembership_passwd_file",
			groupFile:                   "./testdata/groupMembership_group_file",
			expectedFixActionsOnMembers: map[string]fixAction{"zozoPrimaryGroup:user3": addAction, "zozoPrimaryGroup:user4": addAction},
			expectedFixOutput:           ExitOk,
		},

		"with users that are not supposed to be present (del)": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user1", "-user3", "-user2", "-user4", "zozo"},
			},
			passwdFile:                  "./testdata/groupMembership_passwd_file",
			groupFile:                   "./testdata/groupMembership_group_file",
			expectedFixActionsOnMembers: map[string]fixAction{"zozoPrimaryGroup:user1": delAction, "zozoPrimaryGroup:user2": delAction},
			expectedFixOutput:           ExitOk,
		},

		"with a group that does not exist": {
			rule: CompGroupMembership{
				Group:   "IdontExist",
				Members: []string{"-user1", "-user3", "-user2", "-user4", "zozo"},
			},
			passwdFile:                  "./testdata/groupMembership_passwd_file",
			groupFile:                   "./testdata/groupMembership_group_file",
			expectedFixActionsOnMembers: map[string]fixAction{},
			expectedFixOutput:           ExitOk,
		},

		"with users that does not exist": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user1", "lala", "lolo", "-user4", "zozo"},
			},
			passwdFile:                  "./testdata/groupMembership_passwd_file",
			groupFile:                   "./testdata/groupMembership_group_file",
			expectedFixActionsOnMembers: map[string]fixAction{},
			expectedFixOutput:           ExitNok,
		},

		"trying to del a user that has the group as primary group": {
			rule: CompGroupMembership{
				Group:   "zozoPrimaryGroup",
				Members: []string{"-user1", "-user4", "-zozo"},
			},
			passwdFile:                  "./testdata/groupMembership_passwd_file",
			groupFile:                   "./testdata/groupMembership_group_file",
			expectedFixActionsOnMembers: map[string]fixAction{},
			expectedFixOutput:           ExitNok,
		},
	}
	obj := CompGroupsMemberships{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			actionOnMembers := map[string]fixAction{}
			execGetent = func(file string, name string) *exec.Cmd {
				var line string
				switch file {
				case "group":
					fileContent, err := os.ReadFile(c.groupFile)
					require.NoError(t, err)
					line = getLine(fileContent, name)
				case "passwd":
					fileContent, err := os.ReadFile(c.passwdFile)
					require.NoError(t, err)
					line = getLine(fileContent, name)
				default:
					line = `the param file in execGetent only takes as argument "group"" or "file" you should not see this line ! this line come from groupMembership_test.go`
				}
				return exec.Command("echo", "-n", line)
			}
			execId = func(userName string) *exec.Cmd {
				passwdFileContent, err := os.ReadFile(c.passwdFile)
				require.NoError(t, err)
				groupFileContent, err := os.ReadFile(c.groupFile)
				require.NoError(t, err)
				id := getPrimaryGroupId(passwdFileContent, userName)
				return exec.Command("echo", "-n", getGroupNameFromId(groupFileContent, id))
			}
			execUsermodAdd = func(group string, user string) *exec.Cmd {
				actionOnMembers[group+":"+user] = addAction
				return exec.Command("pwd")
			}
			execGpasswdDel = func(group string, user string) *exec.Cmd {
				actionOnMembers[group+":"+user] = delAction
				return exec.Command("pwd")
			}
			require.Equal(t, c.expectedFixOutput, obj.fixRule(c.rule))
			if c.expectedFixOutput == ExitOk {
				require.Equal(t, c.expectedFixActionsOnMembers, actionOnMembers)
			}
		})
	}
}
