package main

import (
	"github.com/stretchr/testify/require"
	"os"
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
