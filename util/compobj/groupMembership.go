package main

import (
	"encoding/json"
	"errors"
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

	execUsermodAdd = func(group string, user string) *exec.Cmd {
		return exec.Command("usermod", "-aG", group, user)
	}

	execGpasswdDel = func(group string, user string) *exec.Cmd {
		return exec.Command("gpasswd", "-d", user, group)
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
	m["group_membership"] = NewCompGroupsMemberShips
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
		t.Errorf("can't check if group exist: %s\n", err)
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
		out = out && t.checkMember(groupMembers, member, rule.Group)
	}
	if out {
		return ExitOk
	}
	return ExitNok
}

func (t CompGroupsMemberships) checkMember(groupMembers map[string]any, member string, groupName string) bool {
	delMember := strings.HasPrefix(member, "-")
	if delMember {
		member = member[1:]
	}
	if primaryGroup, err := t.getPrimaryGroup(member); err != nil {
		t.Errorf("%s\n", err)
		return false
	} else if primaryGroup == groupName {
		if delMember {
			t.VerboseErrorf("user %s has the group %s as primary group and should not be present in the group\n", member, groupName)
			return false
		}
		t.VerboseInfof("user %s has the group %s as primary group and should be present in the group\n", member, groupName)
		return true
	}
	if _, ok := groupMembers[member]; ok {
		if delMember {
			t.VerboseErrorf("user %s is present in the group %s and should not be present\n", member, groupName)
			return false
		}
		t.VerboseInfof("user %s is present in the group %s and should be present\n", member, groupName)
		return true
	}

	if delMember {
		t.VerboseInfof("user %s is not present in the group %s and should not be present\n", member, groupName)
		return true
	}
	t.VerboseErrorf("user %s is not present in the group %s and should be present\n", member, groupName)
	return false
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
		if strings.HasSuffix(member, "\n") {
			member = member[:len(member)-1]
		}
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
			t.Errorf("error when trying to look if user %s exist: %s \n", member, err)
			return ExitNok
		}
		if !isMissing {
			t.Errorf("user %s is missing in the os \n", member)
		}
		missingMembers = missingMembers || isMissing
	}
	if missingMembers {
		return ExitNok
	}
	return ExitOk
}

func (t CompGroupsMemberships) isGroupExisting(groupName string) (bool, error) {
	cmd := execGetent("group", groupName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if exitError.ExitCode() == 2 {
				return false, nil
			}
		}
		return false, fmt.Errorf("can't read /etc/group %w:%s \n", err, output)
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
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if exitError.ExitCode() == 2 {
				return true, nil
			}
		}
		return true, fmt.Errorf("can't read /etc/passwd :%w/%s \n", err, output)
	}
	return false, nil
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

func (t CompGroupsMemberships) fixRule(rule CompGroupMembership) ExitCode {
	isGroupPresent, err := t.isGroupExisting(rule.Group)
	if err != nil {
		t.Errorf("can't check if group %s exist :%s\n", rule.Group, err)
		return ExitNok
	}
	if !isGroupPresent {
		return ExitOk
	}
	e := t.checkMembersExistence(rule.Members)
	if e == ExitNok {
		return ExitNok
	}
	groupMembers, err := t.getGroupMembers(rule.Group)
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	for _, member := range rule.Members {
		if !t.checkMember(groupMembers, member, rule.Group) {
			e = e.Merge(t.fixMember(member, rule.Group))
		}
	}
	return e
}

func (t CompGroupsMemberships) fixMember(member string, group string) ExitCode {
	if strings.HasPrefix(member, "-") {
		member = member[1:]
		return t.fixMemberDel(member, group)
	}
	return t.fixMemberAdd(member, group)
}

func (t CompGroupsMemberships) fixMemberAdd(member string, group string) ExitCode {
	cmd := execUsermodAdd(group, member)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("can't add the user %s to the group %s :%s \n", member, group, output)
		return ExitNok
	}
	t.Infof("adding the user %s to the group %s\n", member, group)
	return ExitOk
}

func (t CompGroupsMemberships) fixMemberDel(member string, group string) ExitCode {
	primaryGroup, err := t.getPrimaryGroup(member)
	if err != nil {
		t.Errorf("can't read primary group for the user %s \n", member)
		return ExitNok
	}
	if group == primaryGroup {
		t.Errorf("user %s has the group %s as primary group, cowardly refusing to del the user from its primary group \n", member, group)
		return ExitNok
	}
	cmd := execGpasswdDel(group, member)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("error when trying to del the user %s from the group %s :%s \n", member, group, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompGroupsMemberships) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompGroupMembership)
		e = e.Merge(t.fixRule(rule))
	}
	return e
}

func (t CompGroupsMemberships) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompGroupsMemberships) Info() ObjInfo {
	return compGroupMembershipInfo
}
