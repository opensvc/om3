package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuthkeyAdd(t *testing.T) {
	testCases := map[string]struct {
		json         string
		expecteError bool
		expectedRule CompAuthKey
	}{
		"with a full add action rule and authfile equal to authorized_keys": {
			json:         `{"action":"add", "authfile":"authorized_keys", "authdir":"/dir/", "user":"toto", "key":"totokey","configfile":"/cf"}`,
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
			json:         `{"action":"del", "authfile":"authorized_keys2", "authdir":"/dir/", "user":"toto", "key":"totokey","port":22,"configfile":"/cf"}`,
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
			json:         `{"action":"lalaal", "authfile":"authorized_keys", "authdir":"/dir", "user":"toto", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no authfile": {
			json:         `{"action":"add", "authfile":"", "authdir":"/dir", "user":"toto", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no user": {
			json:         `{"action":"add", "authfile":"authorized_keys", "authdir":"/dir", "user":"", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no key": {
			json:         `{"action":"add", "authfile":"authorized_keys", "authdir":"/dir", "user":"toto", "key":""}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with a false authfile field (not equal to authorized_keys or authorized_keys2": {
			json:         `{"action":"add", "authfile":"lalala", "authdir":"/dir", "user":"", "key":"totokey"}`,
			expecteError: true,
			expectedRule: CompAuthKey{},
		},

		"json rule with no cf precised": {
			json:         `{"action":"add", "authfile":"authorized_keys", "authdir":"/dir/", "user":"toto", "key":"totokey"}`,
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
