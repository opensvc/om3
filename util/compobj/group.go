package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type (
	CompGroups struct {
		*Obj
	}
	CompGroup struct {
		Group string `json:"group"`
		Gid   *int   `json:"gid"`
	}
)

func pti(i int) *int { return &i }

var (
	execGroupDel = func(groupName string) *exec.Cmd {
		return exec.Command("groupdel", groupName)
	}

	execGroupAdd = func(groupName string, gid int) *exec.Cmd {
		return exec.Command("groupadd", "-g", strconv.Itoa(gid), groupName)
	}

	execChGroupGid = func(groupName string, newGid int) *exec.Cmd {
		return exec.Command("groupmod", "-g", strconv.Itoa(newGid), groupName)
	}

	blacklist = []string{
		"root",
		"bin",
		"daemon",
		"sys",
		"adm",
		"tty",
		"disk",
		"lp",
		"mem",
		"kmem",
		"wheel",
		"mail",
		"uucp",
		"man",
		"games",
		"gopher",
		"video",
		"dip",
		"ftp",
		"lock",
		"audio",
		"nobody",
		"users",
		"utmp",
		"utempter",
		"floppy",
		"vcsa",
		"cdrom",
		"tape",
		"dialout",
		"saslauth",
		"postdrop",
		"postfix",
		"sshd",
		"opensvc",
		"mailnull",
		"smmsp",
		"slocate",
		"rpc",
		"rpcuser",
		"nfsnobody",
		"tcpdump",
		"ntp",
	}

	groupFileContent []byte
	compGroupInfo    = ObjInfo{
		DefaultPrefix: "OSVC_COMP_GROUP_",
		ExampleValue: map[string]CompGroup{
			"group1": {
				Gid: pti(1000),
			},
			"group2": {
				Gid: pti(2000),
			},
		},
		Description: `* Verify a local system group configuration
* A minus (-) prefix to the group name indicates the user should not exist
`,
		FormDefinition: `Desc: |
  A rule defining a list of Unix groups and their properties. Used by the groups compliance objects.
Css: comp48
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: dict of dict
    Key: group
    EmbedKey: No
    Class: group
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
    Id: gid
    Label: Group id
    DisplayModeLabel: gid
    LabelCss: guys16
    Type: string or integer
    Help: The Unix gid of this group.
`,
	}
)

func init() {
	m["group"] = NewCompGroup
}

func NewCompGroup() interface{} {
	return &CompGroups{
		Obj: NewObj(),
	}
}

func (t *CompGroups) Add(s string) error {
	var data map[string]CompGroup
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for groupName, rule := range data {
		if groupName == "" {
			t.Errorf("group should be in the dict: %s\n", s)
			return fmt.Errorf("group should be in the dict: %s\n", s)
		}
		if rule.Gid == nil && !strings.HasPrefix(groupName, "-") {
			t.Errorf("gid should be in the dict: %s\n", s)
			return fmt.Errorf("gid should be in the dict: %s\n", s)
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

func (t CompGroups) locatePresenceInRules(groupName string) int {
	for i, rule := range t.Obj.rules {
		group := rule.(CompGroup)
		if group.Group == groupName || group.Group == "-"+groupName || "-"+group.Group == groupName {
			return i
		}
	}
	return -1
}

func (t CompGroups) checkGroup(rule CompGroup) ExitCode {
	switch strings.HasPrefix(rule.Group, "-") {
	case true:
		rule.Group = rule.Group[1:]
		if t.getGroupGid(rule.Group) == -1 {
			t.VerboseInfof("group %s does not exist and should not exist --> ok\n", rule.Group)
			return ExitOk
		}
		t.VerboseInfof("group %s does exist and should not exist --> not ok\n", rule.Group)
		return ExitNok
	default:
		gid := t.getGroupGid(rule.Group)
		if gid == -1 {
			t.VerboseInfof("group %s does not exist and should exist --> not ok\n", rule.Group)
			return ExitNok
		}
		t.VerboseInfof("group : %s gid = %d target = %d\n", rule.Group, gid, *rule.Gid)
		if gid != *rule.Gid {
			t.VerboseInfof("gid not ok\n")
			return ExitNok
		}
		t.VerboseInfof("gid ok\n")
		return ExitOk
	}
}

func (t CompGroups) getGroupGid(groupName string) int {
	scanner := bufio.NewScanner((bytes.NewReader(groupFileContent)))
	for scanner.Scan() {
		line := scanner.Text()
		splitedLine := strings.Split(line, ":")
		if splitedLine[0] == groupName {
			gid, err := strconv.Atoi(splitedLine[2])
			if err != nil {
				t.Errorf("can't convert gid from /etc/group to int :%s", err)
			}
			return gid
		}
	}
	return -1
}

func (t *CompGroups) Check() ExitCode {
	t.SetVerbose(true)
	e := t.loadGroupFile()
	for _, i := range t.Rules() {
		rule := i.(CompGroup)
		o := t.checkGroup(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompGroups) loadGroupFile() ExitCode {
	var err error
	if !t.checkFileNsswitch() {
		t.Errorf("group is not using files (in /etc/Nsswitch)")
		return ExitNok
	}
	groupFileContent, err = osReadFile("/etc/group")
	if err != nil {
		t.Errorf("can't open /etc/group")
		return ExitNok
	}
	return ExitOk
}

func (t CompGroups) checkFileNsswitch() bool {
	nsswitchFileContent, err := osReadFile("/etc/nsswitch.conf")
	if err != nil {
		t.Errorf("can't open /etc/nsswitch to check if group file are using files :%s", err)
		return false
	}
	scanner := bufio.NewScanner(bytes.NewReader(nsswitchFileContent))

	for scanner.Scan() {
		lineElems := strings.Fields(scanner.Text())
		if len(lineElems) < 1 {
			continue
		}
		if lineElems[0] == "group:" {
			for _, elem := range lineElems {
				if elem == "files" {
					return true
				}
			}
			return false
		}

	}
	return false
}

func (t *CompGroups) Fix() ExitCode {
	t.SetVerbose(false)
	t.loadGroupFile()
	for _, i := range t.Rules() {
		rule := i.(CompGroup)
		if e := t.fixGroup(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}

func (t CompGroups) fixGroup(rule CompGroup) ExitCode {
	if t.checkGroup(rule) == ExitOk {
		return ExitOk
	}
	switch strings.HasPrefix(rule.Group, "-") {
	case true:
		rule.Group = rule.Group[1:]
		return t.fixGroupDel(rule)
	default:
		gid := t.getGroupGid(rule.Group)
		if gid == -1 {
			return t.fixGroupAdd(rule)
		}
		return t.fixGroupGid(rule)
	}

}

func (t CompGroups) fixGroupDel(rule CompGroup) ExitCode {
	for _, groupNameBlackList := range blackList {
		if groupNameBlackList == rule.Group {
			t.Errorf("cowardly refusing to delete group %s \n", rule.Group)
			return ExitNok
		}
	}
	cmd := execGroupDel(rule.Group)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompGroups) fixGroupAdd(rule CompGroup) ExitCode {
	cmd := execGroupAdd(rule.Group, *rule.Gid)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompGroups) fixGroupGid(rule CompGroup) ExitCode {
	cmd := execChGroupGid(rule.Group, *rule.Gid)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompGroups) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompGroups) Info() ObjInfo {
	return compGroupInfo
}
