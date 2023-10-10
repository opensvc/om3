package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type (
	CompGroupsMemberships struct {
		*Obj
	}
	CompGroupMembership struct {
		Group   string   `json:"group"`
		Members []string `json:"members"`
	}
)

var (
	execGetent = func(file string, name string) *exec.Cmd {
		return exec.Command("getent", file, name)
	}
	execId = func(userName string) *exec.Cmd {
		return exec.Command("id", "-gn", userName)
	}
	compGroupMembershipInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_GROUPMEMBERSHIP_",
		ExampleValue: map[string]CompGroupMembership{
			"tibco": {
				Members: []string{"tibco", "tibco1"},
			},
			"tibco1": {
				Members: []string{"tibco1"},
			},
		},
		Description: `* Verify a local system group configuration
* A minus (-) prefix to the group members name indicates the user should not exist
`,
		FormDefinition: `Desc: |
  A rule defining a list of Unix groups and their user membership. The referenced users and groups must exist.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: dict of dict
    Key: group
    EmbedKey: No
    Class: group_membership
Inputs:
  -
    Id: group
    Label: Group name
    DisplayModeLabel: group
    LabelCss: guys16
    Mandatory: Yes
    Type: string
    Help: The Unix group name.
  -
    Id: members
    Label: Group members
    DisplayModeLabel: members
    LabelCss: guy16
    Type: list of string
    Help: A comma-separed list of Unix user names members of this group.
`,
	}
)

func init() {
	m["groupmembership"] = NewCompGroupsMemberShips
}

func NewCompGroupsMemberShips() interface{} {
	return &CompGroupsMemberships{
		Obj: NewObj(),
	}
}

func (t CompGroupsMemberships) Add(s string) error {
	var data map[string]CompGroupMembership
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for groupName, rule := range data {
		if groupName == "" {
			t.Errorf("group should be in the dict: %s\n", s)
			return fmt.Errorf("group should be in the dict: %s\n", s)
		}
		rule.Group = groupName
		if i := t.locatePresenceInRules(rule.Group); i != -1 {
			t.rules[i] = rule
		} else {
			t.Obj.Add(rule)
		}
	}
	return nil
}

func (t CompGroupsMemberships) locatePresenceInRules(groupName string) int {
	for i, rule := range t.Obj.rules {
		group := rule.(CompGroupMembership)
		if group.Group == groupName {
			return i
		}
	}
	return -1
}

func (t CompGroupsMemberships) checkRule(rule CompGroupMembership) ExitCode {
	isGroupPresent, err := t.isGroupExisting(rule.Group)
	if err != nil {
		t.Errorf("can't check if group exist :%s\n", err)
		return ExitNok
	}
	if !isGroupPresent {
		return ExitOk
	}
	if e := t.checkMembersExistence(rule.Members); e == ExitNok {
		return e
	}
	return t.checkGroupMembership(rule)
}

func (t CompGroupsMemberships) checkGroupMembership(rule CompGroupMembership) ExitCode {
	groupMembers, err := t.getGroupMembers(rule.Group)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	out := true
	for _, member := range rule.Members {
		delMember := strings.HasPrefix(member, "-")
		if delMember {
			member = member[1:]
		}
		if primaryGroup, err := t.getPrimaryGroup(member); err != nil {
			t.Errorf("%s\n", err)
			return ExitNok
		} else if primaryGroup == rule.Group {
			if delMember {
				t.VerboseInfof("user %s has the group %s as primary group and should not be present in the group --> not ok\n", member, rule.Group)
				out = false
				continue
			}
			t.VerboseInfof("user %s has the group %s as primary group and should be present in the group --> ok\n", member, rule.Group)
			continue
		}
		if _, ok := groupMembers[member]; ok {
			if delMember {
				t.VerboseInfof("user %s is present in the group %s and should not be present --> not ok\n", member, rule.Group)
				out = false
				continue
			}
			t.VerboseInfof("user %s is present in the group %s and should be present --> ok\n", member, rule.Group)
			continue
		}

		if delMember {
			t.VerboseInfof("user %s is not present in the group %s and should not be present -->  ok\n", member, rule.Group)
			continue
		}
		t.VerboseInfof("user %s is not present in the group %s and should be present --> not ok\n", member, rule.Group)
		out = false
	}
	if out {
		return ExitOk
	}
	return ExitNok
}
func (t CompGroupsMemberships) getGroupMembers(groupName string) (map[string]any, error) {
	m := map[string]any{}
	cmd := execGetent("group", groupName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	lineFields := strings.Split(string(output), ":")
	if len(lineFields) < 4 {
		return nil, fmt.Errorf("error : can't read group members in getent group output command: %s \n", output)
	}
	groupMembersList := strings.Split(lineFields[3], ",")
	for _, member := range groupMembersList {
		m[member] = nil
	}
	return m, nil
}

func (t CompGroupsMemberships) getPrimaryGroup(userName string) (string, error) {
	cmd := execId(userName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w,%s \n", err, output)
	}
	return string(output), nil
}

func (t CompGroupsMemberships) checkMembersExistence(members []string) ExitCode {
	missingMembers := false
	for _, member := range members {
		isMissing, err := t.isUserMissing(member)
		if err != nil {
			t.Errorf("error when trying to look if user %s exist :%s \n", member, err)
			return ExitNok
		}

		missingMembers = missingMembers || isMissing
	}
	if missingMembers {
		t.Errorf("error : some members are not present in the current os\n")
		return ExitNok
	}
	t.VerboseInfof("all the members are user in the current os\n")
	return ExitOk
}

func (t CompGroupsMemberships) isGroupExisting(groupName string) (bool, error) {
	cmd := execGetent("group", groupName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("can't read /etc/group %w:%s \n", err, output)
	}
	if len(output) == 0 {
		return false, nil
	}
	return true, nil
}

func (t CompGroupsMemberships) isUserMissing(member string) (bool, error) {
	if strings.HasPrefix(member, "-") {
		member = member[1:]
	}
	cmd := execGetent("passwd", member)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("can't read /etc/passwd :%w/%s \n", err, output)
	}
	if len(output) == 0 {
		t.Errorf("user %s is missing in the os \n", member)
		return true, nil
	}
	return false, nil
}

func (t CompGroupsMemberships) fixLink(rule CompSymlink) ExitCode {
	/*d := filepath.Dir(rule.Symlink)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		if err := os.MkdirAll(d, 0511); err != nil {
			t.Errorf("symlink: can not create dir %s to host the symlink %s\n", d, rule.Symlink)
			return ExitNok
		}
	}
	err := os.Symlink(rule.Target, rule.Symlink)
	if err != nil {
		t.Errorf("Cant create symlink %s\n", rule.Symlink)
		return ExitNok
	}*/
	return ExitOk
}

func (t CompGroupsMemberships) FixSymlink(rule CompSymlink) ExitCode {
	/*if e := t.CheckSymlink(rule); e == ExitNok {
		if e := t.fixLink(rule); e == ExitNok {
			return e
		}
	}
	return ExitOk*/
	return ExitOk
}

func (t CompGroupsMemberships) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompGroupMembership)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompGroupsMemberships) Fix() ExitCode {
	/*t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		if e := t.FixSymlink(rule); e == ExitNok {
			return ExitNok
		}
	}*/
	return ExitOk
}

func (t CompGroupsMemberships) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompGroupsMemberships) Info() ObjInfo {
	return compGroupMembershipInfo
}
