package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/opensvc/om3/util/file"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
		CheckHome string `json:"check_home"`
	}
)

var (
	shadowFileContent []byte
	passwdFileContent []byte

	blackList = []string{
		"root",
		"bin",
		"daemon",
		"adm",
		"lp",
		"sync",
		"shutdown",
		"halt",
		"mail",
		"news",
		"uucp",
		"operator",
		"nobody",
		"nscd",
		"vcsa",
		"pcap",
		"mailnull",
		"smmsp",
		"sshd",
		"rpc",
		"avahi",
		"rpcuser",
		"nfsnobody",
		"haldaemon",
		"avahi-autoipd",
		"ntp",
	}

	loadFiles = func(t CompUsers) ExitCode {
		var err error
		if !t.checkFilesNsswitch() {
			t.Errorf("shadow or passwd are not using files")
			return ExitNok
		}
		shadowFileContent, err = os.ReadFile("/etc/shadow")
		if err != nil {
			t.Errorf("can't open /etc/shadow : %s", err)
			return ExitNok
		}

		passwdFileContent, err = os.ReadFile("/etc/passwd")
		if err != nil {
			t.Errorf("can't open /etc/passwd : %s", err)
			return ExitNok
		}
		return ExitOk
	}

	execChGid = func(user string, gid int) *exec.Cmd {
		return exec.Command("usermod", "-g", fmt.Sprintf("%d", gid), user)
	}

	execChUid = func(user string, uid int) *exec.Cmd {
		return exec.Command("usermod", "-u", fmt.Sprintf("%d", uid), user)
	}

	execDelCommand = func(user string) *exec.Cmd {
		return exec.Command("bash", `userdel `+user)
	}

	execAddCommand = func(args []string) *exec.Cmd {
		return exec.Command("useradd", args...)
	}

	getHomeDir = func(userInfos []string) string {
		return userInfos[5]
	}

	execAddHomeCommand = func(home string, user string) *exec.Cmd {
		return exec.Command("usermod", "-d", home, user)
	}

	execShellCommand = func(shell string) *exec.Cmd {
		return exec.Command("chsh", "-s", shell)
	}

	execPasswordHashCommand = func(user string, password string) *exec.Cmd {
		return exec.Command("bash", `echo "`+user+`:`+password+`" | chpasswd -e`)
	}

	execGecosCommand = func(gecos string, user string) *exec.Cmd {
		return exec.Command("bash", `echo "`+gecos+`" | chfn `+user)
	}

	osReadFile = os.ReadFile

	compUserInfo = ObjInfo{
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
)

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
			return fmt.Errorf("name should be in the dict: %s\n", s)
		}

		if !strings.HasPrefix(name, "-") {
			if rule.Uid == nil {
				t.Errorf("uid should be in the dict: %s\n", s)
				return fmt.Errorf("uid should be in the dict: %s\n", s)
			}

			if rule.Gid == nil {
				t.Errorf("gid should be in the dict: %s\n", s)
				return fmt.Errorf("gid should be in the dict: %s\n", s)
			}
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
			if u.CheckHome != "yes" {
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

func (t *CompUsers) Check() ExitCode {
	t.SetVerbose(true)
	e := loadFiles(*t)
	if e != ExitOk {
		return e
	}
	for _, i := range t.Rules() {
		rule := i.(CompUser)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompUsers) checkFilesNsswitch() bool {
	nsswitchFileContent, err := osReadFile("/etc/nsswitch.conf")
	var isPasswordInFiles, isShadowInFiles bool
	if err != nil {
		t.Errorf("can't open /etc/nsswitch to check if shadow and password are using files :%s", err)
		return false
	}
	scanner := bufio.NewScanner(bytes.NewReader(nsswitchFileContent))

	for scanner.Scan() {
		lineElems := strings.Fields(scanner.Text())
		if len(lineElems) < 1 {
			continue
		}
		switch lineElems[0] {
		case "passwd:":
			for _, elem := range lineElems {
				if elem == "files" {
					isPasswordInFiles = true
				}
			}
		case "shadow:":
			for _, elem := range lineElems {
				if elem == "files" {
					isShadowInFiles = true
				}
			}
		}

	}
	return isShadowInFiles && isPasswordInFiles
}

func (t CompUsers) checkRule(rule CompUser) ExitCode {
	if strings.HasPrefix(rule.User, "-") {
		_, e := t.checkUserExistence(rule.User, passwdFileContent)
		return e
	}

	userInfos, e := t.checkUserExistence(rule.User, passwdFileContent)

	if e == ExitNok {
		return e
	}

	e = e.Merge(t.checkUserGid(rule, userInfos))
	e = e.Merge(t.checkUserUid(rule, userInfos))
	if rule.Shell != "" {
		e = e.Merge(t.checkUserShell(rule, userInfos))
	}
	if rule.Home != "" {
		e = e.Merge(t.checkUserHomeDir(rule, userInfos))
	}
	if rule.CheckHome == "yes" {
		e = e.Merge(t.checkUserHomeDirOwnerShip(rule, userInfos))
	}
	if rule.Password != "" {
		e = e.Merge(t.checkHash(rule, shadowFileContent))
	}
	if rule.Gecos != "" {
		e = e.Merge(t.checkUserGecos(rule, userInfos))
	}

	return e
}

func (t CompUsers) checkUserGid(rule CompUser, userInfos []string) ExitCode {
	uid, err := t.getGid(userInfos)
	if err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}

	t.Infof("gid = %d target = %d \n", uid, *rule.Uid)
	if uid != *rule.Uid {
		t.Infof("gid not ok \n")
		return ExitNok
	}

	return ExitOk
}

func (t CompUsers) checkUserUid(rule CompUser, userInfos []string) ExitCode {

	gid, err := t.getUid(userInfos)
	if err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}

	t.Infof("gid = %d target = %d \n", gid, *rule.Gid)
	if gid != *rule.Gid {
		t.Infof("gid not ok \n")
		return ExitNok
	}

	return ExitOk
}

func (t CompUsers) checkUserShell(rule CompUser, userInfos []string) ExitCode {
	shell := t.getShell(userInfos)
	t.Infof("user shell = %s target = %s \n", shell, rule.Shell)
	if shell != rule.Shell {
		t.Infof("user shell not ok \n")
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) checkUserHomeDir(rule CompUser, userInfos []string) ExitCode {
	home := getHomeDir(userInfos)
	t.Infof("user home dir = %s target = %s \n", home, rule.Home)
	if !file.Exists(home) {
		t.Infof("user home dir does not exist --> not ok\n")
		return ExitNok
	}
	if home != rule.Home {
		t.Infof("user home not ok \n")
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) checkUserHomeDirOwnerShip(rule CompUser, userInfos []string) ExitCode {
	uid, _, err := file.Ownership(getHomeDir(userInfos))
	if err != nil {
		t.Infof("%s", err)
		return ExitNok
	}
	t.Infof("user home dir owner = %d target = %d \n", uid, *rule.Uid)
	if uid != *rule.Uid {
		t.Infof("user home ownership not ok \n")
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) checkUserGecos(rule CompUser, userInfos []string) ExitCode {
	gecos := t.getGecos(userInfos)
	t.Infof("user gecos = %s target = %s \n", gecos, rule.Gecos)
	if gecos != rule.Gecos {
		t.Infof("user gecos not ok \n")
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) checkHash(rule CompUser, shadow []byte) ExitCode {
	scanner := bufio.NewScanner((bytes.NewReader(shadow)))
	for scanner.Scan() {
		line := scanner.Text()
		splitedLine := strings.SplitN(line, ":", 3)
		if splitedLine[0] == rule.User {
			t.Infof("user password hash = %s target = %s \n", splitedLine[1], rule.Password)
			if splitedLine[1] == rule.Password {
				return ExitOk
			}
			t.Infof("user password hash not ok \n")
			return ExitNok
		}
	}
	t.Infof("not found in /etc/shadow \n")
	return ExitNok
}

func (t CompUsers) getUid(userInfos []string) (int, error) {
	return strconv.Atoi(userInfos[2])
}

func (t CompUsers) getGid(userInfos []string) (int, error) {
	return strconv.Atoi(userInfos[3])
}

func (t CompUsers) getGecos(userInfos []string) string {
	return userInfos[4]
}

func (t CompUsers) getShell(userInfos []string) string {
	return userInfos[6]
}

func (t CompUsers) checkUserExistence(userName string, passwdFile []byte) ([]string, ExitCode) {
	scanner := bufio.NewScanner((bytes.NewReader(passwdFile)))
	checkDel := false
	if strings.HasPrefix(userName, "-") {
		checkDel = true
		userName = userName[1:]
	}
	for scanner.Scan() {
		line := scanner.Text()
		splitedLine := strings.Split(line, ":")
		if splitedLine[0] == userName {
			if checkDel {
				t.VerboseInfof("user %s exist and should not --> not ok\n", userName)
				return splitedLine, ExitNok
			}
			return splitedLine, ExitOk
		}
	}

	if checkDel {
		t.VerboseInfof("user %s doesn't exist --> ok\n", userName)
		return []string{}, ExitOk
	}
	t.VerboseInfof("user %s missing in /etc/passwd \n", userName)
	return []string{}, ExitNok
}

func (t *CompUsers) Fix() ExitCode {
	t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompUser)
		if e := t.fixRule(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}

func (t *CompUsers) fixRule(rule CompUser) ExitCode {
	if loadFiles(*t) == ExitNok {
		return ExitNok
	}
	if strings.HasPrefix(rule.User, "-") {
		_, e := t.checkUserExistence(rule.User, passwdFileContent)
		if e == ExitNok {
			if t.fixUserDel(rule) == ExitNok {
				return ExitNok
			}
			return ExitOk
		}
		return ExitOk
	}

	userInfos, e := t.checkUserExistence(rule.User, passwdFileContent)

	if e == ExitNok {
		if t.fixUserExistence(rule) == ExitNok {
			return ExitNok
		}
		if loadFiles(*t) == ExitNok {
			return ExitNok
		}
		userInfos, e = t.checkUserExistence(rule.User, passwdFileContent)
	}

	if t.checkUserGid(rule, userInfos) == ExitNok {
		if t.fixGid(rule) == ExitNok {
			return ExitNok
		}
	}

	if t.checkUserUid(rule, userInfos) == ExitNok {
		if t.fixUid(rule) == ExitNok {
			return ExitNok
		}
	}

	if rule.Home != "" {
		if t.checkUserHomeDir(rule, userInfos) == ExitNok {
			if t.fixHomeDir(rule) == ExitNok {
				return ExitNok
			}
			if loadFiles(*t) == ExitNok {
				return ExitNok
			}
		}
	}

	if rule.CheckHome == "yes" {
		if t.checkUserHomeDirOwnerShip(rule, userInfos) == ExitNok {
			if t.fixHomeOwnerShip(rule, userInfos) == ExitNok {
				return ExitNok
			}
		}
	}

	if rule.Shell != "" {
		if t.checkUserShell(rule, userInfos) == ExitNok {
			if t.fixShell(rule) == ExitNok {
				return ExitNok
			}
		}
	}

	if rule.Password != "" {
		if t.checkHash(rule, shadowFileContent) == ExitNok {
			if t.fixPasswordHash(rule) == ExitNok {
				return ExitNok
			}
		}
	}

	if rule.Gecos != "" {
		if t.checkUserGecos(rule, userInfos) == ExitNok {
			if t.fixGecos(rule) == ExitNok {
				return ExitNok
			}
		}
	}
	return ExitOk
}

func (t CompUsers) fixGid(rule CompUser) ExitCode {
	cmd := execChGid(rule.User, *rule.Gid)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixUid(rule CompUser) ExitCode {
	cmd := execChUid(rule.User, *rule.Uid)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixUserExistence(rule CompUser) ExitCode {
	cmdArgs := []string{"-u", fmt.Sprintf("%d", *rule.Uid), "-g", fmt.Sprintf("%d", *rule.Gid)}

	if rule.Home != "" {
		cmdArgs = append(cmdArgs, "--home", rule.Home, "--create-home")
	}

	cmd := execAddCommand(cmdArgs)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixHomeOwnerShip(rule CompUser, userInfos []string) ExitCode {
	err := os.Chown(getHomeDir(userInfos), *rule.Uid, -1)
	if err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixHomeDir(rule CompUser) ExitCode {
	err := os.MkdirAll(rule.Home, 0755)
	if err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}
	if file.Exists("/etc/skel") {
		cmd := exec.Command("cp", "-R", "/etc/skel/*", rule.Home)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%s:%s", err, output)
		}
	}

	cmd := execAddHomeCommand(rule.Home, rule.User)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
	}
	return ExitOk
}

func (t CompUsers) fixShell(rule CompUser) ExitCode {

	cmd := execShellCommand(rule.Shell)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s:%s", err, output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixPasswordHash(rule CompUser) ExitCode {
	cmd := execPasswordHashCommand(rule.User, rule.Password)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s", output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixGecos(rule CompUser) ExitCode {
	cmd := execGecosCommand(rule.Gecos, rule.User)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s", output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) fixUserDel(rule CompUser) ExitCode {
	rule.User = rule.User[1:]
	for _, usrBlackList := range blackList {
		if usrBlackList == rule.User {
			t.Errorf("cowardly refusing to delete user : %s", rule.User)
			return ExitNok
		}
	}

	cmd := execDelCommand(rule.User)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s", output)
		return ExitNok
	}
	return ExitOk
}

func (t CompUsers) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompUsers) Info() ObjInfo {
	return compSymlinkInfo
}
