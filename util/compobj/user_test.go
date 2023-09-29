package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserAdd(t *testing.T) {
	pti := func(i int) *int { return &i }
	ptb := func(b bool) *bool { return &b }

	sliceToMap := func(users any) map[string]CompUser {
		m := make(map[string]CompUser)
		switch l := users.(type) {
		case []any:
			for _, user := range l {
				u := user.(CompUser)
				m[u.User] = u
			}
		case []CompUser:
			for _, user := range l {
				m[user.User] = user
			}
		}
		return m
	}
	totoUserData := `"toto" : {"uid" : 10100, "gid" : 10100}`
	totoUserAddFieldsData := `"toto" : {"uid" : 10100, "gid" : 10100,"shell" : "/bin/totoShell","gecos" : "i am toto"}`
	totoCompUser := CompUser{
		User:      "toto",
		Uid:       pti(10100),
		Gid:       pti(10100),
		Shell:     "",
		Home:      "",
		Password:  "",
		Gecos:     "",
		CheckHome: nil,
	}
	totoCompUserFieldAdded := CompUser{
		User:      "toto",
		Uid:       pti(10100),
		Gid:       pti(10100),
		Shell:     "/bin/totoShell",
		Home:      "",
		Password:  "",
		Gecos:     "i am toto",
		CheckHome: nil,
	}
	titiUserData := `"titi" : {"uid" : 10200, "gid" : 10200, "shell" : "/bin/ksh","home" : "/home/titi","password" : "123","gecos" : "i am titi","check_home" : true}`
	titiUserFieldOverWritingData := `"titi" : {"uid" : 10800, "gid" : 10800, "shell" : "/bin/overWriting","home" : "/home/overWriting","password" : "HashOverWriting","gecos" : "i am over-writing titi","check_home" : false}`
	titiCompUser := CompUser{
		User:      "titi",
		Uid:       pti(10200),
		Gid:       pti(10200),
		Shell:     "/bin/ksh",
		Home:      "/home/titi",
		Password:  "123",
		Gecos:     "i am titi",
		CheckHome: ptb(true),
	}
	testCases := map[string]struct {
		jsonRules     []string
		expectedRules []CompUser
	}{
		"with json of 1 rule": {
			jsonRules: []string{
				"{" + totoUserData + "}",
			},
			expectedRules: []CompUser{totoCompUser},
		},

		"with 1 json and many rules in the json (not the same users) :": {
			jsonRules: []string{
				"{" + titiUserData + "," + totoUserData + "}",
			},
			expectedRules: []CompUser{titiCompUser, totoCompUser},
		},

		"with 1 json and many rules in the json (same users) :": {
			jsonRules: []string{
				"{" + titiUserData + "," + titiUserData + "}",
			},
			expectedRules: []CompUser{titiCompUser},
		},

		"with 2 json and 1 rule in each json (not the same users) :": {
			jsonRules: []string{
				"{" + titiUserData + "}", "{" + totoUserData + "}",
			},
			expectedRules: []CompUser{titiCompUser, totoCompUser},
		},

		"with 2 json and 1 rule in each json (same users) :": {
			jsonRules: []string{
				"{" + titiUserData + "}", "{" + titiUserData + "}",
			},
			expectedRules: []CompUser{titiCompUser},
		},

		"with 1 json and 2 same users (with no field over-writing) :": {
			jsonRules: []string{
				"{" + totoUserData + "," + totoUserAddFieldsData + "}",
			},
			expectedRules: []CompUser{totoCompUserFieldAdded},
		},

		"with 2 json and 2 same users (with no field over-writing) :": {
			jsonRules: []string{
				"{" + totoUserData + "}", "{" + totoUserAddFieldsData + "}",
			},
			expectedRules: []CompUser{totoCompUserFieldAdded},
		},

		"with 2 json and 2 same users (with field over-writing) :": {
			jsonRules: []string{
				"{" + titiUserData + "}", "{" + titiUserFieldOverWritingData + "}",
			},
			expectedRules: []CompUser{titiCompUser},
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			obj := CompUsers{&Obj{verbose: true}}
			for _, rule := range test.jsonRules {
				require.NoError(t, obj.Add(rule))
			}
			require.Equal(t, sliceToMap(test.expectedRules), sliceToMap(obj.rules))
		})
	}
}
