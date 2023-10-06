package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserAdd(t *testing.T) {
	pti := func(i int) *int { return &i }

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
	totoUserData := `"toto" : {"uid" : 10100, "gid" : 10100,"check_home" : "no"}`
	totoUserAddFieldsData := `"toto" : {"uid" : 10100, "gid" : 10100,"shell" : "/bin/totoShell","gecos" : "i am toto","check_home" : "no"}`
	totoCompUser := CompUser{
		User:      "toto",
		Uid:       pti(10100),
		Gid:       pti(10100),
		Shell:     "",
		Home:      "",
		Password:  "",
		Gecos:     "",
		CheckHome: "no",
	}
	totoCompUserFieldAdded := CompUser{
		User:      "toto",
		Uid:       pti(10100),
		Gid:       pti(10100),
		Shell:     "/bin/totoShell",
		Home:      "",
		Password:  "",
		Gecos:     "i am toto",
		CheckHome: "no",
	}
	titiUserData := `"titi" : {"uid" : 10200, "gid" : 10200, "shell" : "/bin/ksh","home" : "/home/titi","password" : "123","gecos" : "i am titi","check_home" : "yes"}`
	titiUserFieldOverWritingData := `"titi" : {"uid" : 10800, "gid" : 10800, "shell" : "/bin/overWriting","home" : "/home/overWriting","password" : "HashOverWriting","gecos" : "i am over-writing titi","check_home" : "no"}`
	titiCompUser := CompUser{
		User:      "titi",
		Uid:       pti(10200),
		Gid:       pti(10200),
		Shell:     "/bin/ksh",
		Home:      "/home/titi",
		Password:  "123",
		Gecos:     "i am titi",
		CheckHome: "yes",
	}
	testCases := map[string]struct {
		jsonRules               []string
		expecteErrorInAddOutput bool
		expectedRules           []CompUser
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

		"with a delete rule:": {
			jsonRules: []string{
				`{"-toto" : {}}`,
			},
			expectedRules: []CompUser{{User: "-toto"}},
		},

		"with empty json :": {
			jsonRules:               []string{},
			expecteErrorInAddOutput: true,
		},

		"with missing user name": {
			jsonRules: []string{
				`{"" : {"uid" : 10100, "gid" : 10100,"check_home" : "no"}}`,
			},
			expecteErrorInAddOutput: true,
		},

		"with missing uid": {
			jsonRules: []string{
				`{"toto" : {"gid" : 10100,"check_home" : "no"}}`,
			},
			expecteErrorInAddOutput: true,
		},

		"with missing gid": {
			jsonRules: []string{
				`{"toto" : {"uid" : 10100,"check_home" : "no"}}`,
			},
			expecteErrorInAddOutput: true,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Helper()
			obj := CompUsers{&Obj{verbose: true}}
			for _, rule := range c.jsonRules {
				if c.expecteErrorInAddOutput {
					require.Error(t, obj.Add(rule))
				} else {
					require.NoError(t, obj.Add(rule))
				}
			}
			if c.expecteErrorInAddOutput == false {
				require.Equal(t, sliceToMap(c.expectedRules), sliceToMap(obj.rules))
			}
		})
	}
}

func TestUserCheckFilesNsswitch(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	obj := CompUsers{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}

	TestCases := map[string]struct {
		nsswitchFile   string
		expectedOutput bool
	}{
		"with files for passwd and shadow": {
			nsswitchFile:   "./testdata/user_nsswitch.conf_true",
			expectedOutput: true,
		},

		"with multiple fields but still using files": {
			nsswitchFile:   "./testdata/user_nsswitch.conf_multipleFields",
			expectedOutput: true,
		},

		"with no files for passwd": {
			nsswitchFile:   "./testdata/user_nsswitch.conf_noFilesPasswd",
			expectedOutput: false,
		},

		"with no files for shadow and passwd": {
			nsswitchFile:   "./testdata/user_nsswitch.conf_noFilesPasswd_noFilesShadow",
			expectedOutput: false,
		},

		"with no files for shadow": {
			nsswitchFile:   "./testdata/user_nsswitch.conf_noFilesShadow",
			expectedOutput: false,
		},
	}

	for name, c := range TestCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.nsswitchFile)
			}

			require.Equal(t, c.expectedOutput, obj.checkFilesNsswitch())
		})
	}
}

func TestUserCheckRule(t *testing.T) {
	pti := func(i int) *int { return &i }
	obj := CompUsers{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}

	oriShadowFileContent := shadowFileContent
	defer func() { shadowFileContent = oriShadowFileContent }()

	oriPasswdFileContent := passwdFileContent
	defer func() { passwdFileContent = oriPasswdFileContent }()

	oriGetHomeDir := getHomeDir
	defer func() { getHomeDir = oriGetHomeDir }()

	var err error

	shadowFileContent, err = os.ReadFile("./testdata/user_shadow")
	require.NoError(t, err)

	passwdFileContent, err = os.ReadFile("./testdata/user_passwd")
	require.NoError(t, err)

	withHomePath := func(pRule *CompUser) error {
		tmpDir := t.TempDir()

		getHomeDir = func(userInfos []string) string {
			return tmpDir + oriGetHomeDir(userInfos)
		}

		pRule.Home = tmpDir + pRule.Home
		if err := os.MkdirAll(pRule.Home, 0777); err != nil {
			return err
		}
		return nil
	}

	withHomeOwnerShip := func(pRule *CompUser) error {
		return os.Chown(pRule.Home, *pRule.Uid, 1234)
	}

	withHomeWrongOwnerShip := func(pRule *CompUser) error {
		return os.Chown(pRule.Home, *pRule.Uid+1, 1234)
	}

	DelHomeDirInRule := func(pRule *CompUser) error {
		pRule.Home = ""
		return nil
	}

	testCases := map[string]struct {
		rule           CompUser
		envFunc        []func(pRule *CompUser) error
		needRoot       bool
		expectedOutput ExitCode
	}{
		"check with only mandatory fields": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			expectedOutput: ExitOk,
		},

		"check with all the field": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "yes",
			},
			envFunc:        []func(pRule *CompUser) error{withHomePath, withHomeOwnerShip},
			needRoot:       true,
			expectedOutput: ExitOk,
		},

		"check with wrong uid": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(3000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "no",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check with wrong gid": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(3000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "no",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check with wrong shell": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/wrongShell",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "no",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check with wrong home dir": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/wrongHome",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "no",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check with wrong password hash": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "wrongHash",
				Gecos:     "i am zozo,,,",
				CheckHome: "no",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check with wrong gecos": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am a wrong gecos,,,",
				CheckHome: "no",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check with wrong home dir owner": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "yes",
			},
			envFunc:        []func(pRule *CompUser) error{withHomePath, withHomeWrongOwnerShip},
			needRoot:       true,
			expectedOutput: ExitNok,
		},

		"check home owner ship if the home field the not filled": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "yes",
			},
			envFunc:        []func(pRule *CompUser) error{withHomePath, withHomeWrongOwnerShip, DelHomeDirInRule},
			needRoot:       true,
			expectedOutput: ExitNok,
		},

		"check not supposed to exist but exist": {
			rule: CompUser{
				User:      "-zozo",
				Uid:       nil,
				Gid:       nil,
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitNok,
		},

		"check not supposed to exist and does not exist": {
			rule: CompUser{
				User:      "-iDontExist",
				Uid:       nil,
				Gid:       nil,
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			envFunc:        []func(pRule *CompUser) error{},
			expectedOutput: ExitOk,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			if c.needRoot {
				usr, err := user.Current()
				require.NoError(t, err)
				if usr.Username != "root" {
					t.Skip("need root")
				}
			}
			for _, function := range c.envFunc {
				require.NoError(t, function(&c.rule))
			}
			require.Equal(t, c.expectedOutput, obj.checkRule(c.rule))
		})
	}
}

func TestUserFix(t *testing.T) {
	obj := CompUsers{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}

	oriGetHomeDir := getHomeDir
	defer func() { getHomeDir = oriGetHomeDir }()

	oriLoadFiles := loadFiles
	defer func() { loadFiles = oriLoadFiles }()

	oriExecDelCommand := execDelCommand
	defer func() { execDelCommand = oriExecDelCommand }()

	oriExecAddCommand := execAddCommand
	defer func() { execAddCommand = oriExecAddCommand }()

	oriAddHomeCommand := execAddHomeCommand
	defer func() { execAddHomeCommand = oriAddHomeCommand }()

	oriExecShellCommand := execShellCommand
	defer func() { execShellCommand = oriExecShellCommand }()

	oriExecPasswordHashCommand := execPasswordHashCommand
	defer func() { execPasswordHashCommand = oriExecPasswordHashCommand }()

	oriExecGecosCommand := execGecosCommand
	defer func() { execGecosCommand = oriExecGecosCommand }()

	pti := func(i int) *int { return &i }

	type fixCode int
	var (
		noFix       fixCode = 0
		delFix      fixCode = 1
		addFix      fixCode = 2
		homeFix     fixCode = 3
		shellFix    fixCode = 4
		passwordFix fixCode = 5
		gecosFix    fixCode = 6
	)
	type test struct {
		rule                        CompUser
		currentPasswdFile           string
		currentShadowFile           string
		goldenPasswdFile            string
		goldenShadowFile            string
		envFunction                 []func(pRule *CompUser) error
		needRoot                    bool
		expectedFixAction           fixCode
		FixAction                   fixCode
		expectedFixOutput           ExitCode
		expectedCheckOutputAfterFix ExitCode
	}

	withHomePath := func(pRule *CompUser) error {
		tmpDir := t.TempDir()

		getHomeDir = func(userInfos []string) string {
			return tmpDir + oriGetHomeDir(userInfos)
		}

		pRule.Home = tmpDir + pRule.Home
		fmt.Println("home dir func create :", pRule.Home)
		if err := os.MkdirAll(pRule.Home, 0777); err != nil {
			return err
		}
		return nil
	}

	withWrongHomePath := func(pRule *CompUser) error {
		tmpDir := t.TempDir()

		getHomeDir = func(userInfos []string) string {
			return tmpDir + oriGetHomeDir(userInfos)
		}

		pRule.Home = tmpDir + pRule.Home
		fmt.Println("home dir func create :", pRule.Home)
		if err := os.MkdirAll(pRule.Home+"Wrong", 0777); err != nil {
			return err
		}
		return nil
	}

	withNoHomePath := func(pRule *CompUser) error {
		tmpDir := t.TempDir()

		getHomeDir = func(userInfos []string) string {
			return tmpDir + oriGetHomeDir(userInfos)
		}

		pRule.Home = tmpDir + pRule.Home
		return nil
	}

	withHomeWrongOwnerShip := func(pRule *CompUser) error {
		return os.Chown(pRule.Home, *pRule.Uid+1, 1234)
	}

	withHomeOwnerShip := func(pRule *CompUser) error {
		return os.Chown(pRule.Home, *pRule.Uid, 1234)
	}

	testCases := map[string]test{
		"with a minimum true rule": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 make([]func(pRule *CompUser) error, 0),
			expectedFixAction:           noFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a full true rule": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "yes",
			},
			currentPasswdFile:           "./testdata/user_passwd",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withHomePath, withHomeOwnerShip},
			needRoot:                    true,
			expectedFixAction:           noFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a wrong shell": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_wrong_shell",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withHomePath},
			needRoot:                    false,
			expectedFixAction:           shellFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a wrong home dir (home dir does no exist + wrong)": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_wrong_home",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withNoHomePath},
			needRoot:                    false,
			expectedFixAction:           homeFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a wrong home dir (home dir exist but is wrong)": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_wrong_home",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withWrongHomePath},
			needRoot:                    false,
			expectedFixAction:           homeFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a wrong password hash": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd",
			currentShadowFile:           "./testdata/user_shadow_with_wrong_hash",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withHomePath},
			needRoot:                    false,
			expectedFixAction:           passwordFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a wrong gecos": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_wrong_gecos",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withHomePath},
			needRoot:                    false,
			expectedFixAction:           gecosFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with a wrong home dir ownership": {
			rule: CompUser{
				User:      "zozo",
				Uid:       pti(2000),
				Gid:       pti(2000),
				Shell:     "/bin/bash",
				Home:      "/home/zozo",
				Password:  "zozohash",
				Gecos:     "i am zozo,,,",
				CheckHome: "yes",
			},
			currentPasswdFile:           "./testdata/user_passwd",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{withHomePath, withHomeWrongOwnerShip},
			needRoot:                    true,
			expectedFixAction:           noFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with to zozobis user to be deleted but zozobis is not present": {
			rule: CompUser{
				User:      "-zozobis",
				Uid:       nil,
				Gid:       nil,
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{},
			needRoot:                    false,
			expectedFixAction:           noFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with to zozobis user to be deleted but zozobis is not present (filling other fields with random info)": {
			rule: CompUser{
				User:      "-zozobis",
				Uid:       pti(1234),
				Gid:       pti(1234),
				Shell:     "randomShel",
				Home:      "randomHome",
				Password:  "randomHash",
				Gecos:     "randomecos",
				CheckHome: "yes",
			},
			currentPasswdFile:           "./testdata/user_passwd",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{},
			needRoot:                    false,
			expectedFixAction:           noFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with to zozobis user to be deleted but zozobis is present": {
			rule: CompUser{
				User:      "-zozobis",
				Uid:       pti(1234),
				Gid:       pti(1234),
				Shell:     "randomShel",
				Home:      "randomHome",
				Password:  "randomHash",
				Gecos:     "randomecos",
				CheckHome: "yes",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_zozobis",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{},
			needRoot:                    false,
			expectedFixAction:           delFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"with to zozoadd user to be added ": {
			rule: CompUser{
				User:      "zozoadd",
				Uid:       pti(3000),
				Gid:       pti(3000),
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_zozobis",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{},
			needRoot:                    false,
			expectedFixAction:           addFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitOk,
			expectedCheckOutputAfterFix: ExitOk,
		},

		"try to delete a blacklisted user (daemon) ": {
			rule: CompUser{
				User:      "-daemon",
				Uid:       nil,
				Gid:       nil,
				Shell:     "",
				Home:      "",
				Password:  "",
				Gecos:     "",
				CheckHome: "",
			},
			currentPasswdFile:           "./testdata/user_passwd_with_zozobis",
			currentShadowFile:           "./testdata/user_shadow",
			goldenPasswdFile:            "./testdata/user_passwd",
			goldenShadowFile:            "./testdata/user_shadow",
			envFunction:                 []func(pRule *CompUser) error{},
			needRoot:                    false,
			expectedFixAction:           noFix,
			FixAction:                   noFix,
			expectedFixOutput:           ExitNok,
			expectedCheckOutputAfterFix: ExitNok,
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			if c.needRoot {
				usr, err := user.Current()
				require.NoError(t, err)
				if usr.Username != "root" {
					t.Skip("need root")
				}
			}

			loadFiles = func(t CompUsers) ExitCode {
				var err error
				shadowFileContent, err = os.ReadFile(c.currentShadowFile)
				if err != nil {
					t.Errorf("can't open /etc/shadow : %s", err)
					return ExitNok
				}

				passwdFileContent, err = os.ReadFile(c.currentPasswdFile)
				if err != nil {
					t.Errorf("can't open /etc/passwd : %s", err)
					return ExitNok
				}
				return ExitOk
			}

			execDelCommand = func(user string) *exec.Cmd {
				c.FixAction = delFix
				c.currentPasswdFile = "./testdata/user_passwd"
				c.currentShadowFile = "./testdata/user_shadow"
				return exec.Command("pwd")
			}

			execAddCommand = func(args []string) *exec.Cmd {
				c.FixAction = addFix
				c.currentPasswdFile = "./testdata/user_passwd"
				c.currentShadowFile = "./testdata/user_shadow"
				return exec.Command("pwd")
			}

			execAddHomeCommand = func(home string, user string) *exec.Cmd {
				c.FixAction = homeFix
				c.currentPasswdFile = "./testdata/user_passwd"
				c.currentShadowFile = "./testdata/user_shadow"
				return exec.Command("pwd")
			}

			execShellCommand = func(shell string) *exec.Cmd {
				c.FixAction = shellFix
				c.currentPasswdFile = "./testdata/user_passwd"
				c.currentShadowFile = "./testdata/user_shadow"
				return exec.Command("pwd")
			}

			execPasswordHashCommand = func(user string, password string) *exec.Cmd {
				c.FixAction = passwordFix
				c.currentPasswdFile = "./testdata/user_passwd"
				c.currentShadowFile = "./testdata/user_shadow"
				return exec.Command("pwd")
			}

			execGecosCommand = func(gecos string, user string) *exec.Cmd {
				c.FixAction = gecosFix
				c.currentPasswdFile = "./testdata/user_passwd"
				c.currentShadowFile = "./testdata/user_shadow"
				return exec.Command("pwd")
			}

			for _, f := range c.envFunction {
				require.NoError(t, f(&c.rule))
			}
			require.Equal(t, c.expectedFixOutput, obj.fixRule(c.rule))
			require.Equal(t, c.expectedFixAction, c.FixAction)
			if c.expectedCheckOutputAfterFix == ExitOk {
				require.Equal(t, c.goldenShadowFile, c.currentShadowFile)
				require.Equal(t, c.goldenPasswdFile, c.currentPasswdFile)
			}
			loadFiles(obj)
			require.Equal(t, c.expectedCheckOutputAfterFix, obj.checkRule(c.rule))

		})

	}
}
