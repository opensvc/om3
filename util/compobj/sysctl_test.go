package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"testing"
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
			jsonRules:   `[{"key" : "k","index" : 1,"op": ">>","value": 256}}]`,
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},

		"with a key that is present in live": {
			rule: CompSysctl{
				Key:   "net.ipv4.tcp_l3mdev_accept",
				Index: pti(0),
				Op:    "=",
				Value: float64(0),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file",
			sysctlOutput:        "./testdata/sysctl.out",
			expectedCheckResult: ExitOk,
		},

		"with an index that is out of range": {
			rule: CompSysctl{
				Key:   "net.ipv4.tcp_l3mdev_accept",
				Index: pti(89),
				Op:    "=",
				Value: float64(0),
			},
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
			sysctlConfigFile:    "./testdata/sysctl_config_file",
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
