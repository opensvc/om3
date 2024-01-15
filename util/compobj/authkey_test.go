package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthkeyAdd(t *testing.T) {
	testCases := map[string]struct {
		json                []string
		expectError         bool
		expectBlacklistUser bool
		expectedRule        []CompAuthKey
	}{
		"with a full add action rule and authfile equal to authorized_keys": {
			json:        []string{`{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"}`},
			expectError: false,
			expectedRule: []CompAuthKey{
				{
					Action:     "add",
					Authfile:   "authorized_keys",
					User:       "toto",
					Key:        "totokey",
					ConfigFile: "/cf",
				},
			},
		},

		"with a full del action rule authfile equal to authorized_keys2": {
			json:        []string{`{"action":"del", "authfile":"authorized_keys2", "user":"toto", "key":"totokey","port":22,"configfile":"/cf"}`},
			expectError: false,
			expectedRule: []CompAuthKey{{
				Action:     "del",
				Authfile:   "authorized_keys2",
				User:       "toto",
				Key:        "totokey",
				ConfigFile: "/cf",
			}},
		},

		"with an action that is not correct (not del or add)": {
			json:         []string{`{"action":"lalaal", "authfile":"authorized_keys", "user":"toto", "key":"totokey"}`},
			expectError:  true,
			expectedRule: []CompAuthKey{{}},
		},

		"json rule with no authfile": {
			json:         []string{`{"action":"add", "authfile":"", "user":"toto", "key":"totokey"}`},
			expectError:  true,
			expectedRule: []CompAuthKey{{}},
		},

		"json rule with no user": {
			json:         []string{`{"action":"add", "authfile":"authorized_keys", "user":"", "key":"totokey"}`},
			expectError:  true,
			expectedRule: []CompAuthKey{{}},
		},

		"json rule with no key": {
			json:         []string{`{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":""}`},
			expectError:  true,
			expectedRule: []CompAuthKey{{}},
		},

		"json rule with a false authfile field (not equal to authorized_keys or authorized_keys2": {
			json:         []string{`{"action":"add", "authfile":"lalala", "user":"", "key":"totokey"}`},
			expectError:  true,
			expectedRule: []CompAuthKey{{}},
		},

		"json rule with no cf precised": {
			json:        []string{`{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey"}`},
			expectError: false,
			expectedRule: []CompAuthKey{{
				Action:     "add",
				Authfile:   "authorized_keys",
				User:       "toto",
				Key:        "totokey",
				ConfigFile: "/etc/ssh/sshd_config",
			}},
		},

		"with two same rules ": {
			json:        []string{`{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"}`, `{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"}`},
			expectError: false,
			expectedRule: []CompAuthKey{
				{
					Action:     "add",
					Authfile:   "authorized_keys",
					User:       "toto",
					Key:        "totokey",
					ConfigFile: "/cf",
				},
			},
		},

		"with with one add rule and one del rule for the same key and the same user": {
			json:                []string{`{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"}`, `{"action":"del", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"}`},
			expectError:         false,
			expectBlacklistUser: true,
			expectedRule: []CompAuthKey{
				{
					Action:     "add",
					Authfile:   "authorized_keys",
					User:       "toto",
					Key:        "totokey",
					ConfigFile: "/cf",
				},
			},
		},

		"with a json that is a list of rules": {
			json:        []string{`[{"action":"add", "authfile":"authorized_keys", "user":"toto", "key":"totokey","configfile":"/cf"},{"action":"add", "authfile":"authorized_keys", "user":"toto2", "key":"totokey","configfile":"/cf"}]`},
			expectError: false,
			expectedRule: []CompAuthKey{
				{
					Action:     "add",
					Authfile:   "authorized_keys",
					User:       "toto",
					Key:        "totokey",
					ConfigFile: "/cf",
				}, {
					Action:     "add",
					Authfile:   "authorized_keys",
					User:       "toto2",
					Key:        "totokey",
					ConfigFile: "/cf",
				},
			},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			for _, jsonRule := range c.json {
				err := obj.Add(jsonRule)
				if c.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}
			if c.expectBlacklistUser {
				require.Equal(t, false, userValidityMap[c.expectedRule[0].User])
			}
			if !c.expectError && !c.expectBlacklistUser {
				for i := range c.expectedRule {
					require.Equal(t, c.expectedRule[i], obj.rules[i])
				}
			}
			checkAllowsUsersCfgFile = map[[2]string]any{}
			userValidityMap = map[string]bool{}
			actionKeyUserMap = map[[3]string]any{}
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

	originalUserLookup := userLookup
	defer func() { userLookup = originalUserLookup }()

	originalUserLookGroupID := userLookupGroupID
	defer func() { userLookupGroupID = originalUserLookGroupID }()

	originalOsOpen := osOpen
	defer func() { osOpen = originalOsOpen }()

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

	userLookupGroupID = func(gid string) (*user.Group, error) {
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
			osOpen = func(name string) (*os.File, error) {
				return os.Open(c.sshdConfigFilePath)
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
			// ça ne peut pas marcher
			paths, err := obj.getAuthKeyFilesPaths(c.sshdConfigFilePath, "toto", c.filepath)
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

	oriUserLookup := userLookup
	defer func() { userLookup = oriUserLookup }()

	userLookup = func(username string) (*user.User, error) {
		return nil, nil
	}

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
			getAuthKeyFilesPaths = func(configFilePath string, userName string, authFile string) ([]string, error) {
				return c.authorizedKeysFiles, nil
			}
			require.Equal(t, c.expectedOutput, obj.checkAuthKey(c.rule))
			cacheInstalledKeys = map[string][]string{}
		})
	}
}

func TestAddAuthKey(t *testing.T) {
	oriGetAuthKeyFilePath := getAuthKeyFilePath
	defer func() { getAuthKeyFilePath = oriGetAuthKeyFilePath }()

	makeEnvWithKeyToAdd := func(fileKeyToAddPath string) []string {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "tmpFileKeyToAdd")
		f, err := os.Create(filePath)
		require.NoError(t, err)
		defer func() {
			err := f.Close()
			require.NoError(t, err)
		}()
		fileContent, err := os.ReadFile(fileKeyToAddPath)
		_, err = f.Write(fileContent)
		require.NoError(t, err)
		return []string{filePath}
	}
	testCases := map[string]struct {
		rule                   CompAuthKey
		fileWithKeyToAdd       string
		goldenAuthorizeKeyFile string
	}{
		"add a key in a authorized Key file ": {
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDPiTjBH9tZI59YtzQiMQPMpUzLPfci3p0Eew+pB+pglhkHOxGiCV9abcZDAO8o6mBHlw== lala",
				ConfigFile: "",
			},
			fileWithKeyToAdd:       "./testdata/authkey_authorizedKeyFile_with_key_to_add",
			goldenAuthorizeKeyFile: "./testdata/authkey_sshd_authorizedKeyFile",
		},
	}
	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			filePath := makeEnvWithKeyToAdd(c.fileWithKeyToAdd)
			getAuthKeyFilePath = func(authFile string, configFilePath string, userName string) ([]string, error) {
				return filePath, nil
			}
			obj.addAuthKey(c.rule)
			currentFileContent, err := os.ReadFile(filePath[0])
			require.NoError(t, err)
			goldenFileContent, err := os.ReadFile(c.goldenAuthorizeKeyFile)
			require.NoError(t, err)
			require.Equal(t, string(goldenFileContent), string(currentFileContent))
		})
	}
}

func TestDelAuthKey(t *testing.T) {
	oriGetAuthKeyFilesPaths := getAuthKeyFilesPaths
	defer func() { getAuthKeyFilesPaths = oriGetAuthKeyFilesPaths }()

	makeEnvWithKeyToDel := func(fileKeyToDelPath string) []string {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "tmpFileKeyToDel")
		f, err := os.Create(filePath)
		require.NoError(t, err)
		defer func() {
			err := f.Close()
			require.NoError(t, err)
		}()
		fileContent, err := os.ReadFile(fileKeyToDelPath)
		_, err = f.Write(fileContent)
		require.NoError(t, err)
		return []string{filePath}
	}
	testCases := map[string]struct {
		rule                   CompAuthKey
		fileWithKeyToDel       string
		goldenAuthorizeKeyFile string
	}{
		"del a key in a authorized Key file when the key is at the end of the file": {
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAdeldeldeldeldleldldkzdeleldldeleldledl== delMe",
				ConfigFile: "",
			},
			fileWithKeyToDel:       "./testdata/authkey_authorizedKeyFile_with_key_to_del_end",
			goldenAuthorizeKeyFile: "./testdata/authkey_sshd_authorizedKeyFile",
		},

		"del a key in a authorized Key file when the key is at the middle of the file": {
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "",
				Key:        "ssh-rsa AAAAB3NzaC1yc2EAAdeldeldeldeldleldldkzdeleldldeleldledl== delMe",
				ConfigFile: "",
			},
			fileWithKeyToDel:       "./testdata/authkey_authorizedKeyFile_with_key_to_del_middle",
			goldenAuthorizeKeyFile: "./testdata/authkey_sshd_authorizedKeyFile",
		},
	}
	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			filePath := makeEnvWithKeyToDel(c.fileWithKeyToDel)
			getAuthKeyFilesPaths = func(configFilePath string, userName string, authFile string) ([]string, error) {
				return filePath, nil
			}
			obj.delAuthKey(c.rule)
			currentFileContent, err := os.ReadFile(filePath[0])
			require.NoError(t, err)
			goldenFileContent, err := os.ReadFile(c.goldenAuthorizeKeyFile)
			require.NoError(t, err)
			require.Equal(t, string(goldenFileContent), string(currentFileContent))
		})
	}
}

func TestAddAllowGroup(t *testing.T) {

	oriUserLookGroupID := userLookupGroupID
	defer func() { userLookupGroupID = oriUserLookGroupID }()

	oriUserLookup := userLookup
	defer func() { userLookup = oriUserLookup }()

	makeEnvWithAllowsToAdd := func(fileAllowsToAddPath string) string {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "tmpFileAllowsToAdd")
		f, err := os.Create(filePath)
		require.NoError(t, err)
		defer func() {
			err := f.Close()
			require.NoError(t, err)
		}()
		fileContent, err := os.ReadFile(fileAllowsToAddPath)
		_, err = f.Write(fileContent)
		require.NoError(t, err)
		return filePath
	}

	testCases := map[string]struct {
		rule                CompAuthKey
		fileWithAllowsToAdd string
		goldenAllows        string
	}{
		"with user toto and group totoGroup to be added in sshd config file": {
			rule: CompAuthKey{
				Action:     "",
				Authfile:   "",
				User:       "toto",
				Key:        "",
				ConfigFile: "",
			},
			fileWithAllowsToAdd: "./testdata/authkey_sshd_config_toto_and_totoGroup_to_be_added",
			goldenAllows:        "./testdata/authkey_sshd_config_golden",
		},
	}

	obj := CompAuthkeys{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			userLookupGroupID = func(gid string) (*user.Group, error) {
				group := &user.Group{
					Gid:  "1000",
					Name: "totoGroup",
				}
				return group, nil
			}

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
			c.rule.ConfigFile = makeEnvWithAllowsToAdd(c.fileWithAllowsToAdd)
			obj.addAllowGroups(c.rule)
			obj.addAllowUsers(c.rule)
			currentFileContent, err := os.ReadFile(c.rule.ConfigFile)
			require.NoError(t, err)
			goldenFileContent, err := os.ReadFile(c.goldenAllows)
			require.NoError(t, err)
			require.Equal(t, string(goldenFileContent), string(currentFileContent))
		})
	}
}
