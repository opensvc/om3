package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opensvc/om3/util/file"
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
	fileMD5                 = file.MD5
	tgetParentPid           = CompAuthkeys{}.getParentPid
	checkAllowsUsersCfgFile = map[[2]string]any{}
	userValidityMap         = map[string]bool{}
	actionKeyUserMap        = map[[3]string]any{}
	cacheAllowUsers         []string
	cacheAllowGroups        []string
	cacheInstalledKeys      = map[string][]string{}
	osReadDir               = os.ReadDir
	osReadLink              = os.Readlink
	userLookup              = user.Lookup
	userLookupGroupId       = user.LookupGroupId
	getAuthKeyFilesPaths    = CompAuthkeys{}.getAuthKeyFilesPaths
	getAuthKeyFilePath      = CompAuthkeys{}.getAuthKeyFilePath

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
	m["authkey_list"] = NewCompAutkey
}

func NewCompAutkey() interface{} {
	return &CompAuthkeys{
		Obj: NewObj(),
	}
}

func (t *CompAuthkeys) Add(s string) error {
	var data = []CompAuthKey{{}}
	if err1 := json.Unmarshal([]byte(s), &data[0]); err1 != nil {
		if err2 := json.Unmarshal([]byte(s), &data); err2 != nil {
			return fmt.Errorf("%w: %w", err1, err2)
		}
	}
	for _, rule := range data {
		if err := t.addASingleCompauthkey(rule); err != nil {
			return err
		}
	}
	return nil
}

func (t *CompAuthkeys) addASingleCompauthkey(rule CompAuthKey) error {
	if !(rule.Action == "add" || rule.Action == "del") {
		t.Errorf("action should be equal to add or del in the dict: %s\n", rule)
		return fmt.Errorf("action should be equal to add or del in the dict: %s\n", rule)
	}
	if !(rule.Authfile == "authorized_keys" || rule.Authfile == "authorized_keys2") {
		t.Errorf("authfile should equal to authorized_keys or authorized_keys2 in the dict: %s\n", rule)
		return fmt.Errorf("authfile should equal to authorized_keys or authorized_keys2 in the dict: %s\n", rule)
	}
	if rule.User == "" {
		t.Errorf("user should be in the dict: %s\n", rule)
		return fmt.Errorf("user should be in the dict: %s\n", rule)
	}
	if rule.Key == "" {
		t.Errorf("key should be in the dict: %s\n", rule)
		return fmt.Errorf("user should be in the dict: %s\n", rule)
	}
	if rule.ConfigFile == "" {
		rule.ConfigFile = "/etc/ssh/sshd_config"
	}
	if t.verifyBeforeAdd(rule) {
		t.Obj.Add(rule)
	}
	return nil
}

func (t CompAuthkeys) verifyBeforeAdd(rule CompAuthKey) bool {
	if v, ok := userValidityMap[rule.User]; ok {
		if !v {
			return false
		}
	}
	userValidityMap[rule.User] = true
	switch rule.Action {
	case "add":
		_, ok := actionKeyUserMap[[3]string{"del", rule.Key, rule.User}]
		if ok {
			t.Errorf("the authkeys rules for the user %s generate some conflicts (add and del action for the same key) the user is now blacklisted from check and fix\n", rule.User)
			userValidityMap[rule.User] = false
			return false
		}
		_, ok = actionKeyUserMap[[3]string{"add", rule.Key, rule.User}]
		if ok {
			return false
		}
	case "del":
		_, ok := actionKeyUserMap[[3]string{"add", rule.Key, rule.User}]
		if ok {
			t.Errorf("the authkeys rules for the user %s generate some conflicts (add and del action for the same key) the user is now blacklisted from check and fix\n", rule.User)
			userValidityMap[rule.User] = false
			return false
		}
		_, ok = actionKeyUserMap[[3]string{"del", rule.Key, rule.User}]
		if ok {
			return false
		}
	}
	actionKeyUserMap[[3]string{rule.Action, rule.Key, rule.User}] = nil
	return true
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
	if pid <= 1 {
		panic("arggg")
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
	return tgetParentPid(socketMap[inode])
}

func (t CompAuthkeys) getParentPid(pid int) (int, error) {
	strPid := strconv.Itoa(pid)
	statContent, err := osReadFile(filepath.Join("/proc", strPid, "stat"))
	if err != nil {
		return -1, err
	}
	splitLine := strings.Fields(string(statContent))
	if len(splitLine) < 4 {
		return -1, fmt.Errorf("the stat file of the pid %s, is in the wrong format", strPid)
	}
	strPpid := splitLine[3]
	md5pid, err := fileMD5(filepath.Join("/proc", strPid, "exe"))
	if err != nil {
		return -1, err
	}
	md5Ppid, err := fileMD5(filepath.Join("/proc", strPpid, "exe"))
	if err != nil {
		return -1, err
	}
	if string(md5pid) == string(md5Ppid) {
		ppid, err := strconv.Atoi(strPpid)
		if err != nil {
			return -1, err
		}
		return t.getParentPid(ppid)
	}
	return pid, nil
}

func (t CompAuthkeys) getSocketsMap() (map[int]int, error) {
	socketsMap := map[int]int{}
	files, err := osReadDir("/proc")
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		pid, _ := strconv.Atoi(file.Name())
		if pid == 1 {
			continue
		}
		if file.IsDir() && err == nil {
			fds, err := osReadDir(filepath.Join("/proc", file.Name(), "fd"))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				t.Errorf("%s: can't read proc %s \n", err.Error(), file.Name())
				continue
			}
			for _, fd := range fds {
				link, err := osReadLink(filepath.Join("/proc", file.Name(), "fd", fd.Name()))
				if err != nil {
					if !os.IsNotExist(err) {
						return nil, err
					}
				}
				splitLink := strings.Split(link, "[")
				if splitLink[0] == "socket:" && len(splitLink) == 2 {
					if len(splitLink[1]) > 1 {
						inode, err := strconv.Atoi(splitLink[1][:len(splitLink[1])-1])
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
			tcpFileContent, err := osReadFile(filepath.Join("/proc", file.Name(), "net", "tcp"))
			if err != nil {
				t.Infof("%s: can't read proc %s \n", err.Error(), file.Name())
				continue
			}
			tcp6FileContent, err := osReadFile(filepath.Join("/proc", file.Name(), "net", "tcp6"))
			if err != nil {
				t.Infof("%s: can't read proc %s \n", err.Error(), file.Name())
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
		splitLine := strings.Fields(scanner.Text())
		if len(splitLine) < 10 {
			continue
		}
		splitAdress := strings.Split(splitLine[1], ":")
		if len(splitAdress) != 2 {
			continue
		}
		portUsed, err := strconv.ParseInt(splitAdress[1], 16, 64)
		if err != nil {
			return -1, err
		}
		if int(portUsed) == port {
			inode, err := strconv.Atoi(splitLine[9])
			if err != nil {
				return -1, err
			}
			return inode, nil
		}
	}
	return -1, nil
}

func (t CompAuthkeys) getAuthKeyFilesPaths(configFilePath string, userName string, authFile string) ([]string, error) {
	paths := []string{}
	authKeyList1, err := t.getAuthKeyFilePath("authorized_keys", configFilePath, userName)
	if err != nil {
		return nil, err
	}
	authKeyList2, err := t.readAuthFilePathFromConfigFile(configFilePath, false)
	if err != nil {
		return []string{}, err
	}
	paths = append(paths, authKeyList2...)
	paths = append(paths, authKeyList1...)
	return t.expandPaths(paths, userName)
}

func (t CompAuthkeys) getAuthKeyFilePath(authFile string, configFilePath string, userName string) ([]string, error) {
	if authFile == "authorized_keys" {
		return t.expandPaths([]string{".ssh/authorized_keys"}, userName)
	} else {
		path, err := t.readAuthFilePathFromConfigFile(configFilePath, true)
		if err != nil {
			return []string{}, err
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
		splitLine := strings.Fields(line)
		if splitLine[0] == "AuthorizedKeysFile" && len(splitLine) > 1 {
			if readOnlyTheFirstAuthKeysFile {
				return []string{splitLine[1]}, nil
			}
			return splitLine[1:], nil
		}
	}
	if readOnlyTheFirstAuthKeysFile {
		return []string{".ssh/authorized_keys"}, nil
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

func (t CompAuthkeys) getPortFromConfigFile(configFilePath string) (int, error) {
	fileContent, err := os.ReadFile(configFilePath)
	if err != nil {
		return -1, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(fileContent))
	for scanner.Scan() {
		splitLine := strings.Fields(scanner.Text())
		if len(splitLine) != 2 {
			continue
		}
		if splitLine[0] == "Port" {
			port, err := strconv.Atoi(splitLine[1])
			if err != nil {
				return -1, err
			}
			return port, nil
		}
	}
	return 22, nil
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
		splitLine := strings.Fields(line)
		if splitLine[0] == "AllowUsers" {
			cacheAllowUsers = splitLine[1:]
			return cacheAllowUsers, nil
		}
	}
	//t.VerboseInfof("keyword allowUsers not found \n")
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
		splitLine := strings.Fields(line)
		if splitLine[0] == "AllowGroups" {
			cacheAllowGroups = splitLine[1:]
			return cacheAllowGroups, nil
		}
	}
	//t.VerboseInfof("keyword allowGroups not found \n")
	cacheAllowGroups = []string{"\x00"}
	return cacheAllowGroups, nil
}

func (t CompAuthkeys) getInstalledKeys(configFilePath string, userName string, authFile string) ([]string, error) {
	if _, ok := cacheInstalledKeys[userName]; ok == true {
		return cacheInstalledKeys[userName], nil
	}
	installedKeys := []string{}
	authKeysFiles, err := getAuthKeyFilesPaths(configFilePath, userName, authFile)
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
	_, err := userLookup(rule.User)
	if err != nil {
		switch rule.Action {
		case "add":
			var unknownUserError user.UnknownUserError
			if errors.As(err, &unknownUserError) {
				t.VerboseErrorf("the key %s is not installed and should be installed for the user %s:user does not exist", t.truncateKey(rule.Key), rule.User)
			} else {
				t.Errorf("error when trying to check if user %s exist: %s", rule.User, err)
			}
			return ExitNok
		default:
			var unknownUserError user.UnknownUserError
			if errors.As(err, &unknownUserError) {
				t.VerboseInfof("the key %s is not installed and should be installed for the user %s:user does not exist", t.truncateKey(rule.Key), rule.User)
			} else {
				t.Errorf("error when trying to check if user %s exist: %s", rule.User, err)
			}
			return ExitOk
		}
	}
	installedKeys, err := t.getInstalledKeys(rule.ConfigFile, rule.User, rule.Authfile)
	if err != nil {
		t.Errorf("error when trying to read the authKeys: %s\n", err)
		return ExitNok
	}
	isKeyInstalled := t.isElemInSlice(rule.Key, installedKeys)
	if rule.Action == "add" {
		if isKeyInstalled {
			t.VerboseInfof("the key %s is installed and should be installed for the user %s\n", t.truncateKey(rule.Key), rule.User)
			return ExitOk
		}
		t.VerboseErrorf("the key %s is not installed and should be installed for the user %s\n", t.truncateKey(rule.Key), rule.User)
		return ExitNok
	}
	if isKeyInstalled {
		t.VerboseErrorf("the key %s is installed and should not be installed for the user %s\n", t.truncateKey(rule.Key), rule.User)
		return ExitNok
	}
	t.VerboseInfof("the key %s is not installed and should not be installed for the user %s\n", t.truncateKey(rule.Key), rule.User)
	return ExitOk
}

func (t CompAuthkeys) checkAllowGroups(rule CompAuthKey) ExitCode {
	allowGroups, err := t.getAllowGroups(rule.ConfigFile)
	if err != nil {
		t.Errorf("error when trying to read AllowGroups field in sshd config file: %s\n", err)
		return ExitNok
	}
	if len(allowGroups) > 0 {
		if allowGroups[0] == "\x00" {
			return ExitOk
		}
	}
	primaryGroupName, err := t.getPrimaryGroupName(rule.User)
	if err != nil {
		t.Errorf("can't check the primary group of the user %s: %s\n", rule.User, err)
		return ExitNok
	}
	if t.isElemInSlice(primaryGroupName, allowGroups) {
		t.VerboseInfof("the primary group of the user %s is in AllowGroups in the sshd config file (%s)\n", rule.User, rule.ConfigFile)
		return ExitOk
	}
	t.VerboseErrorf("the primary group of the user %s is not in AllowGroups in the sshd config file (%s)\n", rule.User, rule.ConfigFile)
	return ExitNok
}

func (t CompAuthkeys) getPrimaryGroupName(userName string) (string, error) {
	user1, err := userLookup(userName)
	if err != nil {
		return "", err
	}
	primaryGroup, err := userLookupGroupId(user1.Gid)
	if err != nil {
		return "", err
	}
	return primaryGroup.Name, nil
}

func (t CompAuthkeys) checkAllowUsers(rule CompAuthKey) ExitCode {
	allowUsers, err := t.getAllowUsers(rule.ConfigFile)
	if err != nil {
		t.Errorf("error when trying to read AllowUsers field in sshd config file: %s\n", err)
		return ExitNok
	}
	if len(allowUsers) > 0 {
		if allowUsers[0] == "\x00" {
			return ExitOk
		}
	}
	if t.isElemInSlice(rule.User, allowUsers) {
		t.VerboseInfof("the user %s is in AllowUsers in the sshd config file (%s)\n", rule.User, rule.ConfigFile)
		return ExitOk
	}
	t.VerboseErrorf("the user %s is not in AllowUsers in the sshd config file (%s)\n", rule.User, rule.ConfigFile)
	return ExitNok
}

func (t CompAuthkeys) addAuthKey(rule CompAuthKey) ExitCode {
	authKeyFilePath, err := getAuthKeyFilePath(rule.Authfile, rule.ConfigFile, rule.User)
	if err != nil {
		t.Errorf("error when trying to get the authorized keys file path: %s\n", err)
		return ExitNok
	}
	if len(authKeyFilePath) < 1 {
		t.Errorf("error when trying to get the authorized keys file path\n")
		return ExitNok
	}
	if _, err = os.Stat(authKeyFilePath[0]); err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(filepath.Dir(authKeyFilePath[0]), 0700)
			if err != nil {
				t.Errorf("%s", err)
				return ExitNok
			}
			_, err := os.Create(authKeyFilePath[0])
			if err != nil {
				t.Errorf("%s", err)
				return ExitNok
			}
			if err := os.Chmod(authKeyFilePath[0], 0600); err != nil {
				t.Errorf("%s", err)
				return ExitNok
			}
		} else {
			t.Errorf("%s", err)
			return ExitNok
		}
	}
	f, err := os.OpenFile(authKeyFilePath[0], os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Errorf("error when trying to open : %s to add the key: %s:%s\n", authKeyFilePath[0], t.truncateKey(rule.Key), err)
		return ExitNok
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Errorf("%s", err)
		}
	}()
	_, err = f.Write([]byte(rule.Key + "\n"))
	if err != nil {
		t.Errorf("error when trying to write the key : %s in the file: %s: %s\n", t.truncateKey(rule.Key), authKeyFilePath[0], err)
		return ExitNok
	}
	if _, ok := cacheInstalledKeys[rule.User]; ok {
		cacheInstalledKeys[rule.User] = append(cacheInstalledKeys[rule.User], rule.Key)
	}
	return ExitOk
}

func (t CompAuthkeys) delKeyInFile(authKeyFilePath string, key string) ExitCode {
	configFileNewContent := []byte{}
	configFileOldContent, err := os.ReadFile(authKeyFilePath)
	if err != nil {
		t.Errorf("error when trying to read content of %s: %s\n", authKeyFilePath, err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(configFileOldContent))
	for scanner.Scan() {
		line := scanner.Text()
		lineKey := strings.TrimSpace(line)
		if lineKey == key {
			continue
		}
		configFileNewContent = append(configFileNewContent, []byte(line)...)
		configFileNewContent = append(configFileNewContent, byte('\n'))
	}
	f, err := os.Create(authKeyFilePath)
	if err != nil {
		t.Errorf("can't open the file %s in write mode: %s\n", authKeyFilePath, err)
	}
	if err := os.Chmod(authKeyFilePath, 0600); err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Errorf("error when trying to close file %s: %s\n", authKeyFilePath, err)
		}
	}()
	_, err = f.Write(configFileNewContent)
	if err != nil {
		t.Errorf("error when trying to write in %s: %s\n", authKeyFilePath, err)
	}
	return ExitOk
}

func (t CompAuthkeys) delAuthKey(rule CompAuthKey) ExitCode {
	authKeysFiles, err := getAuthKeyFilesPaths(rule.ConfigFile, rule.User, rule.Authfile)
	if err != nil {
		t.Errorf("error when trying to get the authKey files paths\n")
		return ExitNok
	}
	for _, authKeyFile := range authKeysFiles {
		if t.delKeyInFile(authKeyFile, rule.Key) == ExitNok {
			return ExitNok
		}
	}
	cacheInstalledKeys[rule.User] = delKeyFromCache(rule.Key, cacheInstalledKeys[rule.User])
	return ExitOk
}

func delKeyFromCache(delKey string, keys []string) []string {
	newKeys := []string{}
	for _, key := range keys {
		if key != delKey {
			newKeys = append(newKeys, key)
		}
	}
	return newKeys
}

func (t CompAuthkeys) addAllowGroups(rule CompAuthKey) ExitCode {
	primaryGroupName := ""
	configFileOldContent, err := os.ReadFile(rule.ConfigFile)
	configFileNewContent := []byte{}
	if err != nil {
		t.Errorf("error when trying to read content of %s: %s\n", rule.ConfigFile, err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(configFileOldContent))
	for scanner.Scan() {
		line := scanner.Text()
		splitLine := strings.Fields(line)
		if len(splitLine) > 1 {
			if splitLine[0] == "AllowGroups" {
				primaryGroupName, err = t.getPrimaryGroupName(rule.User)
				if err != nil {
					t.Errorf("can't get the primary group of the user %s: %s\n", rule.User, err)
					return ExitNok
				}
				splitLine = append(splitLine, primaryGroupName)
				configFileNewContent = append(configFileNewContent, []byte(splitLine[0])...)
				for _, elem := range splitLine[1:] {
					configFileNewContent = append(configFileNewContent, []byte(" "+elem)...)
				}
				configFileNewContent = append(configFileNewContent, []byte("\n")...)
				continue
			}
		}
		configFileNewContent = append(configFileNewContent, []byte(line)...)
		configFileNewContent = append(configFileNewContent, []byte("\n")...)
	}
	f, err := os.Create(rule.ConfigFile)
	if err != nil {
		t.Errorf("can't open the file %s in write mode: %s\n", rule.ConfigFile, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Errorf("error when trying to close file %s: %s\n", rule.ConfigFile, err)
		}
	}()
	_, err = f.Write(configFileNewContent)
	if err != nil {
		t.Errorf("error when trying to write in %s: %s\n", rule.ConfigFile, err)
	}
	cacheAllowGroups = append(cacheAllowGroups, primaryGroupName)
	return ExitOk
}

func (t CompAuthkeys) addAllowUsers(rule CompAuthKey) ExitCode {
	configFileOldContent, err := os.ReadFile(rule.ConfigFile)
	configFileNewContent := []byte{}
	if err != nil {
		t.Errorf("error when trying to read content of %s: %s\n", rule.ConfigFile, err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(configFileOldContent))
	for scanner.Scan() {
		line := scanner.Text()
		splitLine := strings.Fields(line)
		if len(splitLine) > 1 {
			if splitLine[0] == "AllowUsers" {
				splitLine = append(splitLine, rule.User)
				configFileNewContent = append(configFileNewContent, []byte(splitLine[0])...)
				for _, elem := range splitLine[1:] {
					configFileNewContent = append(configFileNewContent, []byte(" "+elem)...)
				}
				configFileNewContent = append(configFileNewContent, []byte("\n")...)
				continue
			}
		}
		configFileNewContent = append(configFileNewContent, []byte(line)...)
		configFileNewContent = append(configFileNewContent, []byte("\n")...)
	}
	f, err := os.Create(rule.ConfigFile)
	if err != nil {
		t.Errorf("can't open the file %s in write mode: %s\n", rule.ConfigFile, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Errorf("error when trying to close file %s: %s\n", rule.ConfigFile, err)
		}
	}()
	_, err = f.Write(configFileNewContent)
	if err != nil {
		t.Errorf("error when trying to write in %s: %s\n", rule.ConfigFile, err)
	}
	cacheAllowUsers = append(cacheAllowUsers, rule.User)
	return ExitOk
}

func (t CompAuthkeys) checkRule(rule CompAuthKey) ExitCode {
	if !userValidityMap[rule.User] {
		return ExitNok
	}
	_, err := user.Lookup(rule.User)
	if err != nil {
		if _, ok := err.(user.UnknownUserError); ok {
			switch rule.Action {
			case "add":
				t.VerboseErrorf("the key %s is not installed for the user %s: user does not exist\n", t.truncateKey(rule.Key), rule.User)
				return ExitNok
			case "del":
				t.VerboseInfof("the key %s is not installed for the user %s: user does not exist\n", t.truncateKey(rule.Key), rule.User)
				return ExitOk
			}
		} else {
			t.Errorf("%s \n", err)
			return ExitNok
		}
	}
	e := ExitOk
	e = e.Merge(t.checkAuthKey(rule))
	return e
}

func (t CompAuthkeys) checkAllows() ExitCode {
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompAuthKey)
		_, err := user.Lookup(rule.User)
		if err != nil {
			if _, ok := err.(user.UnknownUserError); !ok {
				t.Errorf("%s", err)
				return ExitNok
			}
			continue
		}
		if rule.Action == "add" {
			if _, ok := checkAllowsUsersCfgFile[[2]string{rule.User, rule.ConfigFile}]; !ok {
				checkAllowsUsersCfgFile[[2]string{rule.User, rule.ConfigFile}] = nil
				e = e.Merge(t.checkAllowGroups(rule))
				e = e.Merge(t.checkAllowUsers(rule))
			}
		}
	}
	return e
}

func (t CompAuthkeys) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompAuthKey)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	e.Merge(t.checkAllows())
	return e
}

func (t CompAuthkeys) fixRule(rule CompAuthKey) ExitCode {
	if !userValidityMap[rule.User] {
		t.Errorf("the user %s is blacklisted can't fix the rule", rule.User)
		return ExitNok
	}
	e := ExitOk
	if t.checkAuthKey(rule) == ExitNok {
		switch rule.Action {
		case "add":
			e = e.Merge(t.addAuthKey(rule))
		case "del":
			e = e.Merge(t.delAuthKey(rule))
		}
	}
	if rule.Action == "add" {
		if t.checkAllowGroups(rule) == ExitNok {
			e = e.Merge(t.addAllowGroups(rule))
		}
		if t.checkAllowUsers(rule) == ExitNok {
			e = e.Merge(t.addAllowUsers(rule))
		}
	}
	port, err := t.getPortFromConfigFile(rule.ConfigFile)
	if err != nil {
		t.Errorf("error when trying to get the port of sshd: %s\n", err)
	} else {
		err = t.reloadSshd(port)
	}
	if err != nil {
		t.Errorf("error when trying to reload sshd: %s\n", err)
	}
	return e
}

func (t CompAuthkeys) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompAuthKey)
		e = e.Merge(t.fixRule(rule))
	}
	return e
}

func (t CompAuthkeys) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompAuthkeys) Info() ObjInfo {
	return compAuthKeyInfo
}
