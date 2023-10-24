package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pbar1/pkill-go"
	"os"
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
	osReadDir  = os.ReadDir
	osReadLink = os.Readlink

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
		t.VerboseInfof("there is no need to reload sshd because sshd is not up")
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
			t.VerboseInfof("info can't convert %s in int in /proc: %s", file.Name(), err)
		}
		if file.IsDir() && err == nil {
			fds, err := osReadDir(filepath.Join("proc", file.Name(), "fd"))
			if err != nil {
				t.Errorf("error:%s can't read proc %s", err.Error(), file.Name())
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
				t.Infof("error:%s can't read proc %s", err.Error(), file.Name())
				continue
			}
			tcp6FileContent, err := osReadFile(filepath.Join("proc", file.Name(), "net", "tcp6"))
			if err != nil {
				t.Infof("error:%s can't read proc %s", err.Error(), file.Name())
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

/*func (t CompAuthkeys) CheckRule(rule CompAuthKey) ExitCode {
	e := ExitOk

	return ExitOk
}

func (t CompAuthkeys) CheckAuthkey(rule CompAuthKey) ExitCode {

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
}*/

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
