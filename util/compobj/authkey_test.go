package main

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestAuthkeyAdd(t *testing.T) {
	testCases := map[string]struct {
		json         string
		expecteError bool
		expectedRule CompAuthKey
	}{
		"with a full add action rule and authfile equal to authorized_keys": {
			json:         `{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"}`,
			expecteError: false,
			expectedRule: CompAuthKey{
				Action:     "add",
				Authfile:   "authorized_keys",
				User:       "toto",
				Key:        "totokey",
				ConfigFile: "/cf",
			},
		},

		"with a full del action rule authfile equal to authorized_keys2": {
			json:         `{"action":"del", "authfile":"authorized_keys2", "user":"toto", "key":"totokey","port":22,"configfile":"/cf"}`,
			expecteError: false,
			expectedRule: CompAuthKey{
				Action:     "del",
				Authfile:   "authorized_keys2",
				User:       "toto",
				Key:        "totokey",
				ConfigFile: "/cf",
			},
		},

		"with an action that is not correct (not del or add)": {
			json:         `{"action":"lalaal", "authfile":"authorized_keys", "user":"toto", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no authfile": {
			json:         `{"action":"add", "authfile":"", "user":"toto", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no user": {
			json:         `{"action":"add", "authfile":"authorized_keys", "user":"", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no key": {
			json:         `{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":""}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with a false authfile field (not equal to authorized_keys or authorized_keys2": {
			json:         `{"action":"add", "authfile":"lalala", "user":"", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no cf precised": {
			json:         `{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey"}`,
			expecteError: false,
			expectedRule: CompAuthKey{
				Action:     "add",
				Authfile:   "authorized_keys",
				User:       "toto",
				Key:        "totokey",
				ConfigFile: "/etc/ssh/sshd_config",
			},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			err := obj.Add(c.json)
			if c.expecteError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, len(obj.rules))
				require.Equal(t, c.expectedRule, obj.rules[0])
			}
		})
	}
}

func TestAuthkeyGetSshdPid(t *testing.T) {
	testCases := map[string]struct {
		testDir     string
		expectError bool
		expectedPid int
	}{
		"the listening tcp and the socket are in the same pid": {
			testDir:     "./testdata/authkey_procDir_with_listening_tcp_in_the_same_pid",
			expectError: false,
			expectedPid: 1,
		},

		"the listening tcp and the socket are not in the same pid": {
			testDir:     "./testdata/authkey_procDir_with_listening_tcp_in_not_the_same_pid",
			expectError: false,
			expectedPid: 1,
		},

		"the listening tcp and the socket are not in the same pid but using tcp6": {
			testDir:     "./testdata/authkey_procDir_with_listening_tcp6_in_the_same_pid",
			expectError: false,
			expectedPid: 1,
		},

		"no listening tcp": {
			testDir:     "./testdata/authkey_procDir_with_no_listening",
			expectError: true,
			expectedPid: -1,
		},
	}
	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			oriOsReadLink := osReadLink
			defer func() { osReadLink = oriOsReadLink }()

			oriOsReadFile := osReadFile
			defer func() { osReadFile = oriOsReadFile }()

			oriOsReadDir := osReadDir
			defer func() { osReadDir = oriOsReadDir }()

			osReadLink = func(name string) (string, error) {
				return os.Readlink(filepath.Join(c.testDir, name))
			}

			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(filepath.Join(c.testDir, name))
			}

			osReadDir = func(name string) ([]os.DirEntry, error) {
				return os.ReadDir(filepath.Join(c.testDir, name))
			}

			pid, err := obj.getSshdPid(22)

			if c.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, c.expectedPid, pid)
		})
	}
}

func TestAuthkeyCheckAllowGroupsCheckAllowUsers(t *testing.T) {
	testCases := map[string]struct {
		sshdConfigFilePath string
		rule               CompAuthKey
		expectedOutput     ExitCode
	}{
		"sshd config file with allows but empty": {
			sshdConfigFilePath: "./testdata/authkey_sshd_config_with_allows_but_empty",
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "toto",
				Key:        "",
				ConfigFile: "",
			},
			expectedOutput: ExitNok,
		},

		"sshd config file with allows but totoGroup and toto are no present in allows": {
			sshdConfigFilePath: "./testdata/authkey_sshd_config_with_allows_no_toto",
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "toto",
				Key:        "",
				ConfigFile: "",
			},
			expectedOutput: ExitNok,
		},

		"sshd config file with allows toto and totoGroup are present but alone in the field": {
			sshdConfigFilePath: "./testdata/authkey_sshd_config_with_allows_only_toto",
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "toto",
				Key:        "",
				ConfigFile: "",
			},
			expectedOutput: ExitOk,
		},

		"sshd config file with allows toto and totoGroup are present and not alone in the field": {
			sshdConfigFilePath: "./testdata/authkey_sshd_config_with_multiples_allows_and_toto",
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "toto",
				Key:        "",
				ConfigFile: "",
			},
			expectedOutput: ExitOk,
		},

		"sshd config file with no allow field": {
			sshdConfigFilePath: "./testdata/authkey_sshd_config_with_no_allows",
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "toto",
				Key:        "",
				ConfigFile: "",
			},
			expectedOutput: ExitOk,
		},
	}

	oriUserLookup := userLookup
	defer func() { userLookup = oriUserLookup }()

	oriUserLookGroupId := userLookupGroupId
	defer func() { userLookupGroupId = oriUserLookGroupId }()

	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	userLookup = func(username string) (*user.User, error) {
		user1 := &user.User{
			Uid:      "1000",
			Gid:      "1000",
			Username: "toto",
			Name:     "toto zozo",
			HomeDir:  "/home/toto",
		}
		return user1, nil
	}

	userLookupGroupId = func(gid string) (*user.Group, error) {
		if gid == "1000" {
			//toto gid
			group := &user.Group{
				Gid:  "1000",
				Name: "totoGroup",
			}
			return group, nil
		}
		return nil, fmt.Errorf("for the test the user should be toto and the gid 1000")
	}

	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.sshdConfigFilePath)
			}
			require.Equal(t, c.expectedOutput, obj.checkAllowUsers(c.rule))
			require.Equal(t, c.expectedOutput, obj.checkAllowGroups(c.rule))
			cacheAllowUsers = nil
			cacheAllowGroups = nil
		})
	}
}

func TestGetAuthKeyFilesPaths(t *testing.T) {
	testCases := map[string]struct {
		filepath           string
		sshdConfigFilePath string
		expectedOutput     []string
	}{
		"with filepath equal to authorized_keys": {
			filepath:           "authorized_keys",
			sshdConfigFilePath: "./testdata/authkey_sshd_config_with_AuthorizedKeysFile",
			expectedOutput:     []string{"/home/toto/.ssh/authorized_keys2", "/home/toto/testTilde", "/home/toto/testH", "/normal/path", "/home/toto/testU", "/home/toto/.ssh/authorized_keys"},
		},
	}

	oriUserLookup := userLookup
	defer func() { userLookup = oriUserLookup }()

	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	userLookup = func(username string) (*user.User, error) {
		user1 := &user.User{
			Uid:      "1000",
			Gid:      "1000",
			Username: "toto",
			Name:     "toto zozo",
			HomeDir:  "/home/toto",
		}
		return user1, nil
	}
	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.sshdConfigFilePath)
			}
			paths, err := obj.getAuthKeyFilesPaths(c.filepath, "toto")
			require.NoError(t, err)
			require.Equal(t, c.expectedOutput, paths)
		})
	}
}

func TestCheckAuthKey(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	oriGetAuthKeyFilesPaths := getAuthKeyFilesPaths
	defer func() { getAuthKeyFilesPaths = oriGetAuthKeyFilesPaths }()

	testCases := map[string]struct {
		rule                CompAuthKey
		authorizedKeysFiles []string
		expectedOutput      ExitCode
	}{
		"with a key that is present and 1 file (action=add)": {
			rule: CompAuthKey{
				Action:     "add",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDPiTjBH9tZIif82+pglhkHOxGiCV9abcZDAO8o6mBHlw== lolo",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile"},
			expectedOutput:      ExitOk,
		},

		"with a key that is present and 1 file (action=del)": {
			rule: CompAuthKey{
				Action:     "del",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDPiTjBH9tZIif82+pglhkHOxGiCV9abcZDAO8o6mBHlw== lolo",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile"},
			expectedOutput:      ExitNok,
		},

		"with a key that is present and 2 files (action=add)": {
			rule: CompAuthKey{
				Action:     "add",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDPiTjBH9tZIif82+pglhkHOxGiCV9abcZDAO8o6mBHlw== lolo",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile", "./testdata/authkey_sshd_authorizedKeyFile2"},
			expectedOutput:      ExitOk,
		},

		"with a key that is present and 2 files (action=del)": {
			rule: CompAuthKey{
				Action:     "del",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDPiTjBH9tZIif82+pglhkHOxGiCV9abcZDAO8o6mBHlw== lolo",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile", "./testdata/authkey_sshd_authorizedKeyFile2"},
			expectedOutput:      ExitNok,
		},

		"with a key that is not present and 1 file (action=add)": {
			rule: CompAuthKey{
				Action:     "add",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa ceheubvniernveu iAmeNot@present",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile"},
			expectedOutput:      ExitNok,
		},

		"with a key that is not present and 1 file (action=del)": {
			rule: CompAuthKey{
				Action:     "del",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa ceheubvniernveu iAmeNot@present",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile"},
			expectedOutput:      ExitOk,
		},

		"with a path file that does not exist": {
			rule: CompAuthKey{
				Action:     "del",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa ceheubvniernveu iAmeNot@present",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{"./testdata/authkey_sshd_authorizedKeyFile", "./testdata/iDontExist"},
			expectedOutput:      ExitOk,
		},

		"with no file path": {
			rule: CompAuthKey{
				Action:     "add",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa ceheubvniernveu iAmeNot@present",
				ConfigFile: "",
			},
			authorizedKeysFiles: []string{},
			expectedOutput:      ExitNok,
		},
	}

	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			getAuthKeyFilesPaths = func(configFilePath string, userName string) ([]string, error) {
				return c.authorizedKeysFiles, nil
			}

			require.Equal(t, c.expectedOutput, obj.checkAuthKey(c.rule))
			cacheInstalledKeys = map[string][]string{}
		})
	}

}
