package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSysctlAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRules     string
		expectedRules []any
		expectError   bool
	}{
		"add with 1 full rule ": {
			jsonRules: `[{"key": "k","index": 1,"op": ">=","value": 256}]`,
			expectedRules: []any{CompSysctl{
				Key:   "k",
				Index: pti(1),
				Op:    ">=",
				Value: float64(256),
			}},
		},

		"add with 2 full rules ": {
			jsonRules: `[{"key": "k","index": 1,"op": ">=","value": 256},{"key": "k2","index": 12,"op": ">=","value": 2562}]`,
			expectedRules: []any{CompSysctl{
				Key:   "k",
				Index: pti(1),
				Op:    ">=",
				Value: float64(256),
			}, CompSysctl{
				Key:   "k2",
				Index: pti(12),
				Op:    ">=",
				Value: float64(2562),
			},
			},
		},

		"add with missing key ": {
			jsonRules:   `[{"index": 1,"op": ">=","value": 256}]`,
			expectError: true,
		},
		"add with missing index ": {
			jsonRules:   `[{"key" : "k","op": ">=","value": 256}]`,
			expectError: true,
		},
		"add with missing op ": {
			jsonRules:   `[{"key" : "k","index" : 1,"value": 256}]`,
			expectError: true,
		},
		"add with missing value ": {
			jsonRules:   `[{"key" : "k","index" : 1,"op": ">="}]`,
			expectError: true,
		},
		"add with wrong op": {
			jsonRules:   `[{"key" : "k","index" : 1,"op": ">>","value": 256}]`,
			expectError: true,
		},
		"add with wrong op ff": {
			jsonRules:   `[{"key" : "k","index" : 1,"op": ">=","value": 256.9}]`,
			expectError: true,
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			if c.expectError {
				require.Error(t, obj.Add(c.jsonRules))
			} else {
				require.NoError(t, obj.Add(c.jsonRules))
				require.Equal(t, c.expectedRules, obj.Obj.rules)
			}
		})
	}
}

func TestSysctlCheck(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	oriExecSysctl := execSysctl
	defer func() { execSysctl = oriExecSysctl }()
	testCases := map[string]struct {
		rule                CompSysctl
		sysctlConfigFile    string
		sysctlOutput        string
		expectedCheckResult ExitCode
	}{
		"with a key that is not present": {
			rule: CompSysctl{
				Key:   "i.am.not.present",
				Index: pti(3),
				Op:    "=",
				Value: 0,
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitNok,
		},

		"with a key that is present in the conf": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(0),
				Op:    "=",
				Value: float64(3),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},

		"with a key that respect the rule in the config file but that is not the same in live parameters": {
			rule: CompSysctl{
				Key:   "iamnotthesameinlive",
				Index: pti(0),
				Op:    "=",
				Value: float64(0),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitNok,
		},

		"with an index that is out of range": {
			rule: CompSysctl{
				Key:   "net.ipv4.tcp_l3mdev_accept",
				Index: pti(89),
				Op:    "=",
				Value: float64(0),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitNok,
		},

		"with an index that is not 0": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "=",
				Value: float64(1),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},

		"with a wrong value and operator =": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "=",
				Value: float64(9),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitNok,
		},

		"with a wrong value and operator <=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "<=",
				Value: float64(0),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitNok,
		},

		"with a wrong value and operator >=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    ">=",
				Value: float64(2),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitNok,
		},

		"with a true rule and operator <=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "<=",
				Value: float64(2),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},

		"with a true rule and operator >=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    ">=",
				Value: float64(0),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},

		"with value equal to a string and operator =": {
			rule: CompSysctl{
				Key:   "kernel.domainname",
				Index: pti(0),
				Op:    "=",
				Value: "example.com",
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file_golden",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},
	}
	obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.sysctlConfigFile)
			}
			execSysctl = func(key string) *exec.Cmd {
				return exec.Command("cat", c.sysctlOutput)
			}
			require.Equal(t, c.expectedCheckResult, obj.checkRule(c.rule))
		})
	}
}

func TestModifyKeyInConfFile(t *testing.T) {
	oriSysctlConfigFilePath := sysctlConfigFilePath
	defer func() { sysctlConfigFilePath = oriSysctlConfigFilePath }()

	testCases := map[string]struct {
		rule                   CompSysctl
		sysctlConfigFile       string
		sysctlConfigFileGolden string
		expectChange           bool
	}{
		"modification at the index 0": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(0),
				Op:    "=",
				Value: 3.0,
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_mofication_index_0",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           true,
		},

		"modification at the index 1": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(1),
				Op:    "=",
				Value: 4.0,
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_mofication_index_1",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           true,
		},

		"modification at the index 3": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(3),
				Op:    "=",
				Value: 3.0,
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_mofication_index_3",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           true,
		},

		"modification with multiples same keys": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(0),
				Op:    "=",
				Value: 3.0,
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_multiple_keys",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           true,
		},

		"try to do modification with a key that is not present": {
			rule: CompSysctl{
				Key:   "iAmNotPresent",
				Index: pti(0),
				Op:    "=",
				Value: 3.0,
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_golden",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           false,
		},

		"try to do modification with a key that is not an int": {
			rule: CompSysctl{
				Key:   "kernel.domainname",
				Index: pti(0),
				Op:    "=",
				Value: "example.com",
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_mofication_not_int",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           true,
		},

		"with an out of range index": {
			rule: CompSysctl{
				Key:   "kernel.domainname",
				Index: pti(9),
				Op:    "=",
				Value: "example.com",
			},
			sysctlConfigFile:       "./testdata/sysctl_config_file_golden",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectChange:           false,
		},
	}

	makeEnvWithKeyToAdd := func(fileKeyToModifyPath string) string {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "tmpFileKeyToModify")
		f, err := os.Create(filePath)
		require.NoError(t, err)
		defer func() {
			err := f.Close()
			require.NoError(t, err)
		}()
		fileContent, err := os.ReadFile(fileKeyToModifyPath)
		_, err = f.Write(fileContent)
		require.NoError(t, err)
		return filePath
	}
	obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			sysctlConfigFilePath = makeEnvWithKeyToAdd(c.sysctlConfigFile)
			change, err := obj.modifyKeyInConfFile(c.rule)
			require.NoError(t, err)
			require.Equal(t, c.expectChange, change)
			content, err := os.ReadFile(sysctlConfigFilePath)
			require.NoError(t, err)
			goldenContent, err := os.ReadFile(c.sysctlConfigFileGolden)
			require.NoError(t, err)
			require.Equal(t, string(goldenContent), string(content))
		})
	}
}

func TestAddKeyInConfFile(t *testing.T) {
	oriSysctlConfigFilePath := sysctlConfigFilePath
	defer func() { sysctlConfigFilePath = oriSysctlConfigFilePath }()

	oriExecSysctl := execSysctl
	defer func() { execSysctl = oriExecSysctl }()

	makeEnvWithKeyToAdd := func(fileKeyToModifyPath string) string {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "tmpFileKeyToModify")
		f, err := os.Create(filePath)
		require.NoError(t, err)
		defer func() {
			err := f.Close()
			require.NoError(t, err)
		}()
		fileContent, err := os.ReadFile(fileKeyToModifyPath)
		_, err = f.Write(fileContent)
		require.NoError(t, err)
		return filePath
	}
	obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	testCases := map[string]struct {
		rule                   CompSysctl
		sysctlOutput           string
		sysctlConfigFile       string
		sysctlConfigFileGolden string
		expectError            bool
	}{
		"add with index 0": {
			rule: CompSysctl{
				Key:   "test",
				Index: pti(0),
				Op:    "=",
				Value: 1.0,
			},
			sysctlOutput:           "./testdata/sysctl.out",
			sysctlConfigFile:       "./testdata/sysctl_config_file_add",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectError:            false,
		},

		"add a key that does not exist": {
			rule: CompSysctl{
				Key:   "iDontExist",
				Index: pti(0),
				Op:    "=",
				Value: 1.0,
			},
			sysctlOutput:           "./testdata/sysctl.out",
			sysctlConfigFile:       "./testdata/sysctl_config_file_add",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectError:            true,
		},

		"add with an index out of range": {
			rule: CompSysctl{
				Key:   "test",
				Index: pti(99),
				Op:    "=",
				Value: 1.0,
			},
			sysctlOutput:           "./testdata/sysctl.out",
			sysctlConfigFile:       "./testdata/sysctl_config_file_add",
			sysctlConfigFileGolden: "./testdata/sysctl_config_file_golden",
			expectError:            true,
		},
	}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			execSysctl = func(key string) *exec.Cmd {
				return exec.Command("cat", c.sysctlOutput)
			}
			sysctlConfigFilePath = makeEnvWithKeyToAdd(c.sysctlConfigFile)
			if c.expectError {
				require.Error(t, obj.addKeyInConfFile(c.rule))
			} else {
				require.NoError(t, obj.addKeyInConfFile(c.rule))
				content, err := os.ReadFile(sysctlConfigFilePath)
				require.NoError(t, err)
				goldenContent, err := os.ReadFile(c.sysctlConfigFileGolden)
				require.NoError(t, err)
				require.Equal(t, string(goldenContent), string(content))
			}
		})
	}
}

func TestSysctlCheckForFix(t *testing.T) {
	oriOsReadFile := osReadFile
	defer func() { osReadFile = oriOsReadFile }()

	oriExecSysctl := execSysctl
	defer func() { execSysctl = oriExecSysctl }()
	testCases := map[string]struct {
		rule                       CompSysctl
		sysctlConfigFile           string
		sysctlOutput               string
		expectedCheckResult        ExitCode
		expectedIsKeyPresentResult bool
	}{
		"with a key that is not present": {
			rule: CompSysctl{
				Key:   "i.am.not.present",
				Index: pti(3),
				Op:    "=",
				Value: 0,
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitNok,
			expectedIsKeyPresentResult: false,
		},

		"with a key that is present in the conf": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(0),
				Op:    "=",
				Value: float64(3),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitOk,
			expectedIsKeyPresentResult: true,
		},

		"with a key that is present in live": {
			rule: CompSysctl{
				Key:   "net.ipv4.tcp_l3mdev_accept",
				Index: pti(0),
				Op:    "=",
				Value: float64(0),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitNok,
			expectedIsKeyPresentResult: true,
		},

		"with an index that is out of range": {
			rule: CompSysctl{
				Key:   "net.ipv4.tcp_l3mdev_accept",
				Index: pti(89),
				Op:    "=",
				Value: float64(0),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitNok,
			expectedIsKeyPresentResult: true,
		},

		"with an index that is not 0": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "=",
				Value: float64(1),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitOk,
			expectedIsKeyPresentResult: true,
		},

		"with a wrong value and operator =": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "=",
				Value: float64(9),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitNok,
			expectedIsKeyPresentResult: true,
		},

		"with a wrong value and operator <=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "<=",
				Value: float64(0),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitNok,
			expectedIsKeyPresentResult: true,
		},

		"with a wrong value and operator >=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    ">=",
				Value: float64(2),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitNok,
			expectedIsKeyPresentResult: true,
		},

		"with a true rule and operator <=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    "<=",
				Value: float64(2),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitOk,
			expectedIsKeyPresentResult: true,
		},

		"with a true rule and operator >=": {
			rule: CompSysctl{
				Key:   "kernel.printk",
				Index: pti(2),
				Op:    ">=",
				Value: float64(0),
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitOk,
			expectedIsKeyPresentResult: true,
		},

		"with value equal to a string and operator =": {
			rule: CompSysctl{
				Key:   "kernel.domainname",
				Index: pti(0),
				Op:    "=",
				Value: "example.com",
			},
			sysctlConfigFile:           "./testdata/sysctl_config_file_golden",
			sysctlOutput:               "./testdata/sysctl.out",
			expectedCheckResult:        ExitOk,
			expectedIsKeyPresentResult: true,
		},
	}
	obj := CompSysctls{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			osReadFile = func(name string) ([]byte, error) {
				return os.ReadFile(c.sysctlConfigFile)
			}
			execSysctl = func(key string) *exec.Cmd {
				return exec.Command("cat", c.sysctlOutput)
			}
			e, isKeyPresent := obj.checkRuleForFix(c.rule)
			require.Equal(t, c.expectedCheckResult, e)
			require.Equal(t, c.expectedIsKeyPresentResult, isKeyPresent)
			fmt.Println(isKeyPresent)
		})
	}
}
