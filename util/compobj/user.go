package main

import (
	"encoding/json"
)

type (
	CompUsers struct {
		*Obj
	}

	CompUser struct {
		User      string `json:"-"`
		Uid       *int   `json:"uid"`
		Gid       *int   `json:"gid"`
		Shell     string `json:"shell"`
		Home      string `json:"home"`
		Password  string `json:"password"`
		Gecos     string `json:"gecos"`
		CheckHome *bool  `json:"check_home"`
	}
)

var compUserInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_USER_",
	ExampleValue: map[string]CompUser{
		"user1": {
			Shell: "/bin/ksh",
			Gecos: "a gecos",
		},
		"user2": {
			Shell: "/bin/ksh",
			Gecos: "another gecos",
		},
	},
	Description: `* Verify a local system user configuration
* A minus (-) prefix to the user name indicates the user should not exist
`,
	FormDefinition: `Desc: |
  A rule defining a list of Unix users and their properties. Used by the users and group_membership compliance objects.
Css: comp48
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: dict of dict
    Key: user
    EmbedKey: No
    Class: user
Inputs:
  -
    Id: user
    Label: User name
    DisplayModeLabel: user
    LabelCss: guy16
    Mandatory: Yes
    Type: string
    Help: The Unix user name.
  -
    Id: uid
    Label: User id
    DisplayModeLabel: uid
    LabelCss: guy16
    Mandatory: Yes
    Type: integer
    Help: The Unix uid of this user.
  -
    Id: gid
    Label: Group id
    DisplayModeLabel: gid
    LabelCss: guys16
    Mandatory: Yes
    Type: integer
    Help: The Unix principal gid of this user.
  -
    Id: shell
    Label: Login shell
    DisplayModeLabel: shell
    LabelCss: action16
    Type: string
    Help: The Unix login shell for this user.
  -
    Id: home
    Label: Home directory
    DisplayModeLabel: home
    LabelCss: action16
    Type: string
    Help: The Unix home directory full path for this user.
  -
    Id: password
    Label: Password hash
    DisplayModeLabel: pwd
    LabelCss: action16
    Type: string
    Help: The password hash for this user. It is recommanded to set it to '!!' or to set initial password to change upon first login. Leave empty to not check nor set the password.
  -
    Id: gecos
    Label: Gecos
    DisplayModeLabel: gecos
    LabelCss: action16
    Type: string
    Help: A one-line comment field describing the user.
  -
    Id: check_home
    Label: Enforce homedir ownership
    DisplayModeLabel: home ownership
    LabelCss: action16
    Type: string
    Default: yes
    Candidates:
      - "yes"
      - "no"
    Help: Toggles the user home directory ownership checking.
`,
}

func init() {
	m["user"] = NewCompUsers
}

func NewCompUsers() interface{} {
	return &CompUsers{
		Obj: NewObj(),
	}
}

func (t *CompUsers) Add(s string) error {
	var data map[string]CompUser
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for name, rule := range data {

		if name == "" {
			t.Errorf("name should be in the dict: %s\n", s)
			return nil
		}

		if rule.Uid == nil {
			t.Errorf("uid should be in the dict: %s\n", s)
			return nil
		}

		if rule.Gid == nil {
			t.Errorf("gid should be in the dict: %s\n", s)
			return nil
		}
		i, b := t.hasUserRule(name)
		if b {
			u := t.rules[i].(CompUser)
			if u.Gecos == "" {
				u.Gecos = rule.Gecos
			}
			if u.Shell == "" {
				u.Shell = rule.Shell
			}
			if u.Home == "" {
				u.Home = rule.Home
			}
			if u.Password == "" {
				u.Password = rule.Password
			}
			if u.CheckHome == nil {
				u.CheckHome = rule.CheckHome
			}

			t.rules[i] = u
		} else {
			rule.User = name
			t.Obj.Add(rule)
		}

	}
	return nil
}

func (t *CompUsers) hasUserRule(userName string) (int, bool) {
	for i, rule := range t.Rules() {
		rule := rule.(CompUser)
		if rule.User == userName {
			return i, true
		}
	}
	return -1, false
}

func (t CompUsers) Check() ExitCode {
	t.SetVerbose(true)
	//e := ExitOk
	/*for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		o := t.CheckSymlink(rule)
		e = e.Merge(o)
	}
	return e*/
	return ExitOk
}

func (t CompUsers) Fix() ExitCode {
	t.SetVerbose(false)
	/*
		for _, i := range t.Rules() {
			rule := i.(CompSymlink)
			/*if e := t.FixSymlink(rule); e == ExitNok {
				return ExitNok
			}
		}*/
	return ExitOk
}

func (t CompUsers) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompUsers) Info() ObjInfo {
	return compSymlinkInfo
}
