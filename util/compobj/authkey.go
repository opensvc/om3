package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pbar1/pkill-go"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type (
	CompAuthkeys struct {
		*Obj
	}
	CompAuthKey struct {
		Action     string `json:"action"`
		Authfile   string `json:"authfile"`
		User       string `json:"user"`
		Key        string `json:"key"`
		ConfigFile string `json:"configfile"`
	}
)

var (
	cacheAllowUsers      []string
	cacheAllowGroups     []string
	cacheInstalledKeys   = map[string][]string{}
	osReadDir            = os.ReadDir
	osReadLink           = os.Readlink
	userLookup           = user.Lookup
	userLookupGroupId    = user.LookupGroupId
	getAuthKeyFilesPaths = CompAuthkeys{}.getAuthKeyFilesPaths

	compAuthKeyInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_AUTHKEY_",
		ExampleValue: CompAuthKey{
			Action:   "add",
			Authfile: "authorized_keys",
			User:     "testuser",
			Key:      "ssh-dss AAAAB3NzaC1kc3MAAACBAPiO1jlT+5yrdPLfQ7sYF52NkfCEzT0AUUNIl+14Sbkubqe+TcU7U3taUtiDJ5YOGOzIVFIDGGtwD0AqNHQbvsiS1ywtC5BJ9362FlrpVH4o1nVZPvMxRzz5hgh3HjxqIWqwZDx29qO8Rg1/g1Gm3QYCxqPFn2a5f2AUiYqc1wtxAAAAFQC49iboZGNqssicwUrX6TUrT9H0HQAAAIBo5dNRmTF+Vd/+PI0JUOIzPJiHNKK9rnySlaxSDml9hH2LuDSjYz7BWuNP8UnPOa2pcFA4meDp5u8d5dGOWxkuYO0bLnXwDZuHtDW/ySytjwEaBLPxoqRBAyfyQNlusGsuiqDYRA7j7bS0RxINBxvDw79KdyQhuOn8/lKVG+sjrQAAAIEAoShly/JlGLQxQzPyWADV5RFlaRSPaPvFzcYT3hS+glkVd6yrCbzc30Yc8Ndu4cflQiXSZzRoUMgsy5PzuiH1M8JjwHTGNl8r9OfJpnN/OaAhMpIyA06y1ZZD9iEME3UmthFQoZnfRuE3yxi7bqyXJU4rOq04iyCTpU1UKInPdXQ= testuser",
		},
		Description: `* Installs or removes ssh public keys from authorized_key files
* Looks up the authorized_key and authorized_key2 file location in the running sshd daemon configuration.
* Add user to sshd_config AllowUser and AllowGroup if used
* Reload sshd if sshd_config has been changed
`,
		FormDefinition: `Desc: |
  Describe a list of ssh public keys to authorize login as the specified Unix user.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: dict
    Class: authkey

Inputs:
  -
    Id: action
    Label: Action
    DisplayModeLabel: action
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Candidates:
      - add
      - del
    Help: Defines wether the public key must be installed or uninstalled.

  -
    Id: user
    Label: User
    DisplayModeLabel: user
    LabelCss: guy16
    Mandatory: Yes
    Type: string
    Help: Defines the Unix user name who will accept those ssh public keys.

  -
    Id: key
    Label: Public key
    DisplayModeLabel: key
    LabelCss: guy16
    Mandatory: Yes
    Type: text
    DisplayModeTrim: 60
    Help: The ssh public key as seen in authorized_keys files.

  -
    Id: authfile
    Label: Authorized keys file name
    DisplayModeLabel: authfile
    LabelCss: hd16
    Mandatory: Yes
    Candidates:
      - authorized_keys
      - authorized_keys2
    Default: authorized_keys2
    Type: string
    Help: The authorized_keys file to write the keys into.

  -
    Id: configfile
    Label: sshd config file path
    DisplayModeLabel: configfile
    LabelCss: hd16
    Mandatory: no
    Default: /etc/ssh/sshd_config
    Type: string
    Help: The sshd configuration file path, if not precised the value used is /etc/ssh/sshd_config
`,
	}
)

func init() {
	m["authkey"] = NewCompAutkey
}

func NewCompAutkey() interface{} {
	return &CompAuthkeys{
		Obj: NewObj(),
	}
}

func (t *CompAuthkeys) Add(s string) error {
	var data CompAuthKey
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	if !(data.Action == "add" || data.Action == "del") {
		t.Errorf("action should be equal to add or del in the dict: %s\n", s)
		return fmt.Errorf("action should be equal to add or del in the dict: %s\n", s)
	}
	if !(data.Authfile == "authorized_keys" || data.Authfile == "authorized_keys2") {
		t.Errorf("authfile should equal to authorized_keys or authorized_keys2 in the dict: %s\n", s)
		return fmt.Errorf("authfile should equal to authorized_keys or authorized_keys2 in the dict: %s\n", s)
	}
	if data.User == "" {
		t.Errorf("user should be in the dict: %s\n", s)
		return fmt.Errorf("user should be in the dict: %s\n", s)
	}
	if data.Key == "" {
		t.Errorf("key should be in the dict: %s\n", s)
		return fmt.Errorf("user should be in the dict: %s\n", s)
	}
	if data.ConfigFile == "" {
		data.ConfigFile = "/etc/ssh/sshd_config"
	}
	t.Obj.Add(data)
	return nil
}

func (t CompAuthkeys) truncateKey(key string) string {
	if len(key) < 50 {
		return key
	}
	return fmt.Sprintf("'%s ... %s'", key[0:17], key[len(key)-30:])
}

func (t CompAuthkeys) reloadSshd(port int) error {
	pids, err := pkill.Pgrep("sshd")
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		t.VerboseInfof("there is no need to reload sshd because sshd is not up \n")
		return nil
	}
	pid, err := t.getSshdPid(port)
	if err != nil {
		return err
	}
	err = syscall.Kill(pid, syscall.SIGHUP)
	if err != nil {
		return err
	}
	return nil
}

func (t CompAuthkeys) getSshdPid(port int) (int, error) {
	socketMap, err := t.getSocketsMap()
	if err != nil {
		return -1, err
	}
	inode, err := t.getInodeListeningOnPort(port)
	if err != nil {
		return -1, err
	}
	return socketMap[inode], nil
}

func (t CompAuthkeys) getSocketsMap() (map[int]int, error) {
	socketsMap := map[int]int{}
	files, err := osReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		pid, err := strconv.Atoi(file.Name())
		if err != nil {
			t.VerboseInfof("info can't convert %s in int in /proc: %s \n", file.Name(), err)
		}
		if file.IsDir() && err == nil {
			fds, err := osReadDir(filepath.Join("proc", file.Name(), "fd"))
			if err != nil {
				t.Errorf("error:%s can't read proc %s \n", err.Error(), file.Name())
				continue
			}
			for _, fd := range fds {
				link, err := osReadLink(filepath.Join("proc", file.Name(), "fd", fd.Name()))
				if err != nil {
					return nil, err
				}
				splitedLink := strings.Split(link, "[")
				if splitedLink[0] == "socket:" && len(splitedLink) == 2 {
					if len(splitedLink[1]) > 1 {
						inode, err := strconv.Atoi(splitedLink[1][:len(splitedLink[1])-2])
						if err != nil {
							return nil, err
						}
						socketsMap[inode] = pid
					}
				}
			}
		}
	}
	return socketsMap, nil
}

func (t CompAuthkeys) getInodeListeningOnPort(port int) (int, error) {
	files, err := osReadDir("/proc")
	if err != nil {
		return -1, nil
	}
	for _, file := range files {
		if file.IsDir() && err == nil {
			tcpFileContent, err := osReadFile(filepath.Join("proc", file.Name(), "net", "tcp"))
			if err != nil {
				t.Infof("error:%s can't read proc %s \n", err.Error(), file.Name())
				continue
			}
			tcp6FileContent, err := osReadFile(filepath.Join("proc", file.Name(), "net", "tcp6"))
			if err != nil {
				t.Infof("error:%s can't read proc %s \n", err.Error(), file.Name())
				continue
			}

			inode, err := t.getInodeFromTcpFileContent(port, tcpFileContent)
			if err != nil {
				return -1, err
			}
			if inode != -1 {
				return inode, nil
			}

			inode, err = t.getInodeFromTcpFileContent(port, tcp6FileContent)
			if err != nil {
				return -1, err
			}
			if inode != -1 {
				return inode, nil
			}
		}
	}
	return -1, fmt.Errorf("there is no process listening on port %d", port)
}

func (t CompAuthkeys) getInodeFromTcpFileContent(port int, content []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		splitedLine := strings.Fields(scanner.Text())
		if len(splitedLine) < 10 {
			continue
		}
		splitedAdress := strings.Split(splitedLine[1], ":")
		if len(splitedAdress) != 2 {
			continue
		}
		portUsed, err := strconv.ParseInt(splitedAdress[1], 16, 64)
		if err != nil {
			return -1, err
		}
		if int(portUsed) == port {
			inode, err := strconv.Atoi(splitedLine[9])
			if err != nil {
				return -1, err
			}
			return inode, nil
		}
	}
	return -1, nil
}

func (t CompAuthkeys) getAuthKeyFilesPaths(configFilePath string, userName string) ([]string, error) {
	paths := []string{}
	authKeyList1, err := t.getAuthKeyFilePath("authorized_keys", configFilePath, userName)
	if err != nil {
		return nil, err
	}
	if configFilePath == "authorized_keys" {
		authKeyList2, err := t.readAuthFilePathFromConfigFile(configFilePath, false)
		if err != nil {
			return []string{}, err
		}
		paths = append(paths, authKeyList2...)
	}
	paths = append(paths, authKeyList1...)
	return t.expandPaths(paths, userName)
}

func (t CompAuthkeys) getAuthKeyFilePath(configFile string, configFilePath string, userName string) ([]string, error) {
	if configFile == "authorized_keys" {
		return t.expandPaths([]string{".ssh/authorized_keys"}, userName)
	} else {
		path, err := t.readAuthFilePathFromConfigFile(configFilePath, true)
		if err != nil {
			return []string{}, nil
		}
		return t.expandPaths(path, userName)
	}
}

func (t CompAuthkeys) readAuthFilePathFromConfigFile(configFilePath string, readOnlyTheFirstAuthKeysFile bool) ([]string, error) {
	configFileContent, err := osReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(configFileContent))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		splitedLine := strings.Fields(line)
		if splitedLine[0] == "AuthorizedKeysFile" && len(splitedLine) > 1 {
			if readOnlyTheFirstAuthKeysFile {
				return []string{splitedLine[1]}, nil
			}
			return splitedLine[1:], nil
		}
	}
	return []string{".ssh/authorized_keys2"}, nil
}

func (t CompAuthkeys) expandPaths(paths []string, userName string) ([]string, error) {
	expandedPaths := []string{}
	user1, err := userLookup(userName)
	if err != nil {
		return []string{}, err
	}
	for _, path := range paths {
		path = strings.Replace(path, "%u", userName, -1)
		path = strings.Replace(path, "~", "~"+userName, -1)
		path = strings.Replace(path, "%h", "~"+userName, -1)
		if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "~") {
			path = filepath.Join("~"+userName, path)
		}
		path = strings.Replace(path, "~"+userName, user1.HomeDir, -1)
		expandedPaths = append(expandedPaths, path)
	}
	return expandedPaths, nil
}

func (t CompAuthkeys) getAllowUsers(sshdConfigFilePath string) ([]string, error) {
	if cacheAllowUsers != nil {
		return cacheAllowUsers, nil
	}
	sshdConfigFileContent, err := osReadFile(sshdConfigFilePath)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(sshdConfigFileContent))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		splitedLine := strings.Fields(line)
		if splitedLine[0] == "AllowUsers" {
			cacheAllowUsers = splitedLine[1:]
			return cacheAllowUsers, nil
		}
	}
	t.VerboseInfof("no allowUsers field find in the sshd config \n")
	cacheAllowUsers = []string{"\x00"}
	return cacheAllowUsers, nil
}

func (t CompAuthkeys) getAllowGroups(sshdConfigFilePath string) ([]string, error) {
	if cacheAllowGroups != nil {
		return cacheAllowGroups, nil
	}
	sshdConfigFileContent, err := osReadFile(sshdConfigFilePath)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(sshdConfigFileContent))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 1 {
			continue
		}
		splitedLine := strings.Fields(line)
		if splitedLine[0] == "AllowGroups" {
			cacheAllowGroups = splitedLine[1:]
			return cacheAllowGroups, nil
		}
	}
	t.VerboseInfof("no allowGroups field find in the sshd config \n")
	cacheAllowGroups = []string{"\x00"}
	return cacheAllowGroups, nil
}

func (t CompAuthkeys) getInstalledKeys(configFilePath string, userName string) ([]string, error) {
	if _, ok := cacheInstalledKeys[userName]; ok == true {
		return cacheInstalledKeys[userName], nil
	}
	installedKeys := []string{}
	authKeysFiles, err := getAuthKeyFilesPaths(configFilePath, userName)
	if err != nil {
		return nil, err
	}
	for _, filePath := range authKeysFiles {
		fileContent, err := osReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		installedKeys = append(installedKeys, strings.Split(string(fileContent), "\n")...)
	}
	cacheInstalledKeys[userName] = installedKeys
	return cacheInstalledKeys[userName], nil
}

func (t CompAuthkeys) isElemInSlice(elem string, slice []string) bool {
	for _, elemS := range slice {
		if elemS == elem {
			return true
		}
	}
	return false
}

func (t CompAuthkeys) checkAuthKey(rule CompAuthKey) ExitCode {
	installedKeys, err := t.getInstalledKeys(rule.ConfigFile, rule.User)
	if err != nil {
		t.Errorf("error when trying to read the authKeys :%s", err)
		return ExitNok
	}
	isKeyInstalled := t.isElemInSlice(rule.Key, installedKeys)
	if rule.Action == "add" {
		if isKeyInstalled {
			t.VerboseInfof("the key %s is installed and should be installed --> ok\n", t.truncateKey(rule.Key))
			return ExitOk
		}
		t.VerboseInfof("the key %s is not installed and should be installed --> not ok\n", t.truncateKey(rule.Key))
		return ExitNok
	}
	if isKeyInstalled {
		t.VerboseInfof("the key %s is installed and should not be installed --> not ok\n", t.truncateKey(rule.Key))
		return ExitNok
	}
	t.VerboseInfof("the key %s is not installed and should not be installed --> ok\n", t.truncateKey(rule.Key))
	return ExitOk
}

func (t CompAuthkeys) checkAllowGroups(rule CompAuthKey) ExitCode {
	allowGroups, err := t.getAllowGroups(rule.ConfigFile)
	if err != nil {
		t.Errorf("error when trying to read allowGroups field in sshd config file :%s\n", err)
		return ExitNok
	}
	if len(allowGroups) > 0 {
		if allowGroups[0] == "\x00" {
			return ExitOk
		}
	}
	user1, err := userLookup(rule.User)
	if err != nil {
		t.Errorf("can't check the primary group of the user %s: %s\n", rule.User, err)
		return ExitNok
	}
	primaryGroup, err := userLookupGroupId(user1.Gid)
	if err != nil {
		t.Errorf("can't check the primary group of the user %s: %s\n", rule.User, err)
		return ExitNok
	}
	primaryGroupName := primaryGroup.Name
	if t.isElemInSlice(primaryGroupName, allowGroups) {
		t.VerboseInfof("the primary group of the user %s is in allowGroups in the sshd config file\n", rule.User)
		return ExitOk
	}
	t.VerboseInfof("the primary group of the user %s is not in allowGroups in the sshd config file\n", rule.User)
	return ExitNok
}

func (t CompAuthkeys) checkAllowUsers(rule CompAuthKey) ExitCode {
	allowUsers, err := t.getAllowUsers(rule.ConfigFile)
	if err != nil {
		t.Errorf("error when trying to read allowUsers field in sshd config file :%s\n", err)
		return ExitNok
	}
	if len(allowUsers) > 0 {
		if allowUsers[0] == "\x00" {
			return ExitOk
		}
	}
	if t.isElemInSlice(rule.User, allowUsers) {
		t.VerboseInfof("the user %s is in allowUsers in the sshd config file\n", rule.User)
		return ExitOk
	}
	t.VerboseInfof("the user %s is not in allowUsers in the sshd config file\n", rule.User)
	return ExitNok
}

func (t CompAuthkeys) CheckRule(rule CompAuthKey) ExitCode {
	e := ExitOk
	e = e.Merge(t.checkAuthKey(rule))
	if rule.Action == "add" {
		e = e.Merge(t.checkAllowGroups(rule))
		e = e.Merge(t.checkAllowUsers(rule))
	}
	return e
}

func (t CompAuthkeys) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompAuthKey)
		o := t.CheckRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompAuthkeys) Fix() ExitCode {
	/*t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		if e := t.FixSymlink(rule); e == ExitNok {
			return ExitNok
		}
	}*/
	return ExitOk
}

func (t CompAuthkeys) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompAuthkeys) Info() ObjInfo {
	return compAuthKeyInfo
}
