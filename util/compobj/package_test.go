package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackage_loadInstalledPackages(t *testing.T) {
	type (
		simulateEnvFunc func(CmdOutputFilePath string)
	)

	dpkgEnv := func(cmdOutputFilePath string) {

		osVendor = "ubuntu"
		osName = "linux"

		cmdRun = func(r commandInterface) error {
			return nil
		}

		cmdStdout = func(r commandInterface) []byte {
			b, _ := os.ReadFile(cmdOutputFilePath)
			return b
		}

		execLookPath = func(path string) (string, error) {
			if path == "dpkg" {
				return "path", nil
			}
			return "", nil
		}
	}

	rpmEnv := func(cmdOutputFilePath string) {

		osVendor = "redhat"
		osName = "linux"
		cmdRun = func(r commandInterface) error {
			return nil
		}

		cmdStdout = func(r commandInterface) []byte {
			b, _ := os.ReadFile(cmdOutputFilePath)
			return b
		}

		execLookPath = func(path string) (string, error) {
			if path == "rpm" {
				return "path", nil
			}
			return "", nil
		}

	}

	pkgaddEnv := func(cmdOutputFilePath string) {

		osVendor = "solaris"
		osName = "sunos"
		cmdRun = func(r commandInterface) error {
			return nil
		}

		cmdStdout = func(r commandInterface) []byte {
			b, _ := os.ReadFile(cmdOutputFilePath)
			return b
		}

		execLookPath = func(path string) (string, error) {
			if path == "pkgadd" {
				return "path", nil
			}
			return "", nil
		}
	}

	freebsdpkgEnv := func(cmdOutputFilePath string) {

		osVendor = "freebsd"
		osName = "freebsd"
		cmdRun = func(r commandInterface) error {
			return nil
		}

		cmdStdout = func(r commandInterface) []byte {
			b, _ := os.ReadFile(cmdOutputFilePath)
			return b
		}

		execLookPath = func(path string) (string, error) {
			if path == "pkg" {
				return "path", nil
			}
			return "", nil
		}
	}

	testCases := map[string]struct {
		environment               simulateEnvFunc
		cmdOutputDataPath         string
		expectedPackagesMapOutput map[string]interface{}
	}{
		"read packages with dpkg": {
			environment:               dpkgEnv,
			cmdOutputDataPath:         "./testdata/cmdUbuntuInstalledPackages.out",
			expectedPackagesMapOutput: map[string]interface{}{"accountsservice": nil, "acl": nil, "acpi-support": nil, "bind9-dnsutils": nil},
		},

		"read packages with rpm": {
			environment:               rpmEnv,
			cmdOutputDataPath:         "./testdata/cmdRedHatInstalledPackages.out",
			expectedPackagesMapOutput: map[string]interface{}{"make": nil, "make.x86_64": nil, "yum-metadata-parser": nil, "yum-metadata-parser.x86_64": nil, "nss-tools": nil, "nss-tools.x86_64": nil, "tar": nil, "tar.x86_64": nil},
		},

		"read packages with pkgadd (solaris) ": {
			environment:               pkgaddEnv,
			cmdOutputDataPath:         "./testdata/cmdSolarisInstalledPackages.out",
			expectedPackagesMapOutput: map[string]interface{}{"SUNWsmhba": nil, "SUNWsmhbar": nil, "SUNWsmpd": nil},
		},

		"read packages with pkg (freeBSD) ": {
			environment:               freebsdpkgEnv,
			cmdOutputDataPath:         "./testdata/cmdFreeBSDInstalledPackages.out",
			expectedPackagesMapOutput: map[string]interface{}{"ca_root_nss": nil, "curl": nil, "pkg": nil},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				hasItMap = map[string]bool{}
				packages = map[string]interface{}{}
			}()

			origCmdRun := cmdRun
			origCmdStdout := cmdStdout
			origExecLookPath := execLookPath
			c.environment(c.cmdOutputDataPath)
			defer func() {
				cmdStdout = origCmdStdout
				cmdRun = origCmdRun
				execLookPath = origExecLookPath
			}()

			require.NoError(t, loadInstalledPackages())
			require.Equal(t, c.expectedPackagesMapOutput, packages)
		})
	}
}

func TestPackage_checkRule(t *testing.T) {
	testCases := map[string]struct {
		rule              CompPackage
		packagesInstalled map[string]interface{}
		expectedResult    ExitCode
	}{
		"all the packages fit to the rule": {
			rule:              CompPackage{"acpi", "tar", "zip"},
			packagesInstalled: map[string]interface{}{"acpi": nil, "tar": nil, "zip": nil},
			expectedResult:    ExitOk,
		},
		"more packages than necessary": {
			rule:              CompPackage{"acpi", "tar", "zip"},
			packagesInstalled: map[string]interface{}{"acpi": nil, "tar": nil, "zip": nil, "foo": nil, "foo2": nil},
			expectedResult:    ExitOk,
		},
		"missing some packages": {
			rule:              CompPackage{"acpi", "tar", "zip"},
			packagesInstalled: map[string]interface{}{"acpi": nil},
			expectedResult:    ExitNok,
		},
		"missing all packages": {
			rule:              CompPackage{"acpi", "tar", "zip"},
			packagesInstalled: map[string]interface{}{"foo": nil, "bar": nil},
			expectedResult:    ExitNok,
		},
		"with 1 excluded packages present": {
			rule:              CompPackage{"-acpi", "tar", "-zip"},
			packagesInstalled: map[string]interface{}{"acpi": nil, "tar": nil},
			expectedResult:    ExitNok,
		},
		"with 2 excluded packages present": {
			rule:              CompPackage{"-acpi", "tar", "-zip"},
			packagesInstalled: map[string]interface{}{"acpi": nil, "tar": nil, "zip": nil},
			expectedResult:    ExitNok,
		},
		"with all excluded packages present": {
			rule:              CompPackage{"-acpi", "-tar", "-zip"},
			packagesInstalled: map[string]interface{}{"acpi": nil, "tar": nil, "zip": nil},
			expectedResult:    ExitNok,
		},
		"excluded packages are not installed": {
			rule:              CompPackage{"-acpi", "tar", "-zip"},
			packagesInstalled: map[string]interface{}{"tar": nil, "foobar": nil},
			expectedResult:    ExitOk,
		},
		"empty rule": {
			rule:              CompPackage{},
			packagesInstalled: map[string]interface{}{"tar": nil, "foobar": nil},
			expectedResult:    ExitOk,
		},
	}

	obj := CompPackages{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			packagesOri := packages
			defer func() { packages = packagesOri }()

			packages = c.packagesInstalled
			require.Equal(t, c.expectedResult, obj.CheckRule(c.rule))
		})
	}
}

func TestPackage_expand(t *testing.T) {

	var expandFunc func(names []string) ([]string, error)

	cmdRunOri := cmdRun
	defer func() { cmdRun = cmdRunOri }()

	cmdStdoutOri := cmdStdout
	defer func() { cmdStdout = cmdStdoutOri }()

	osArchOri := osArch
	defer func() { osArch = osArchOri }()

	createEnv := func(cmdOutputFilePath string, arch string, pkgMgr string) {

		osArch = arch

		cmdRun = func(r commandInterface) error {
			return nil
		}

		cmdStdout = func(r commandInterface) []byte {
			b, _ := os.ReadFile(cmdOutputFilePath)
			return b
		}

		expandFunc = func(names []string) ([]string, error) {
			switch pkgMgr {
			case "apt":
				return aptExpand(names)
			case "yum":
				return yumExpand(names)
			case "dnf":
				return dnfExpand(names)
			default:
				panic("can't create env for unexpected package manager " + pkgMgr)
			}
		}
	}

	sliceToMap := func(s []string) map[string]interface{} {
		m := map[string]interface{}{}
		for _, elem := range s {
			m[elem] = nil
		}
		return m
	}

	testCases := map[string]struct {
		pkgSys            string
		arch              string
		cmdOutputDataPath string
		pkgNames          []string
		expectedPkgList   []string
	}{
		"testing the parser aptExpand": {
			pkgSys:            "apt",
			arch:              "",
			cmdOutputDataPath: "./testdata/cmdUbuntuListPackages",
			pkgNames:          []string{"xsol", "zip", "zzuf"},
			expectedPkgList:   []string{"xsol", "zip", "zzuf"},
		},

		"testing the parser yumExpand with no specified arch in pkgNames but no choice for bash (64 bits arch) ": {
			pkgSys:            "yum",
			arch:              "amd64",
			cmdOutputDataPath: "./testdata/cmdRedHatListPackages",
			pkgNames:          []string{"bash", "systemd-container", "bash-completion", "iproute"},
			expectedPkgList:   []string{"bash.x86_64", "systemd-container.noarch", "iproute.amd64", "bash-completion.noarch", "bash-completion.amd64"},
		},

		"testing the parser yumExpand with a 32 bits arch and multiple systemd-container 32bits arch": {
			pkgSys:            "yum",
			arch:              "i586",
			cmdOutputDataPath: "./testdata/cmdRedHatListPackages",
			pkgNames:          []string{"bash", "systemd-container", "bash-completion", "iproute"},
			expectedPkgList:   []string{"bash.x86_64", "systemd-container.noarch", "bash-completion.noarch"},
		},

		"testing the parser yumExpand with only specified arch in pkgNames, systemd-container is not specify but only 32bits arch is available": {
			pkgSys:            "yum",
			arch:              "i586",
			cmdOutputDataPath: "./testdata/cmdRedHatListPackagesSpecified.out",
			pkgNames:          []string{"bash.x86_64", "systemd-container", "bash-completion.amd64", "iproute.amd64"},
			expectedPkgList:   []string{"bash.x86_64", "systemd-container.i686", "bash-completion.amd64", "iproute.amd64"},
		},

		"testing the parser dnfExpand with no specified arch in pkgNames but no choice for bash (64 bits arch) ": {
			pkgSys:            "dnf",
			arch:              "amd64",
			cmdOutputDataPath: "./testdata/cmdRedHatListPackages",
			pkgNames:          []string{"bash", "systemd-container", "bash-completion", "iproute"},
			expectedPkgList:   []string{"bash.x86_64", "systemd-container.noarch", "iproute.amd64", "bash-completion.noarch", "bash-completion.amd64"},
		},

		"testing the parser dnfExpand with a 32 bits arch and multiple systemd-container 32bits arch": {
			pkgSys:            "dnf",
			arch:              "i586",
			cmdOutputDataPath: "./testdata/cmdRedHatListPackages",
			pkgNames:          []string{"bash", "systemd-container", "bash-completion", "iproute"},
			expectedPkgList:   []string{"bash.x86_64", "systemd-container.noarch", "bash-completion.noarch"},
		},

		"testing the parser dnfExpand with only specified arch in pkgNames, systemd-container is not specify but only 32bits arch is available": {
			pkgSys:            "dnf",
			arch:              "i586",
			cmdOutputDataPath: "./testdata/cmdRedHatListPackagesSpecified.out",
			pkgNames:          []string{"bash.x86_64", "systemd-container", "bash-completion.amd64", "iproute.amd64"},
			expectedPkgList:   []string{"bash.x86_64", "systemd-container.i686", "bash-completion.amd64", "iproute.amd64"},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			createEnv(c.cmdOutputDataPath, c.arch, c.pkgSys)
			list, err := expandFunc(c.pkgNames)
			require.NoError(t, err)
			require.Equal(t, sliceToMap(c.expectedPkgList), sliceToMap(list))
		})
	}
}

func TestAdd(t *testing.T) {
	testCases := map[string]struct {
		jsonRules     []string
		expectedRules []interface{}
	}{
		"with a simple rule": {
			jsonRules:     []string{`["lalal","yoyo"]`},
			expectedRules: []interface{}{CompPackage{"lalal", "yoyo"}},
		},
		"with a two simple rules": {
			jsonRules:     []string{`["lalal","yoyo"]`, `["toto"]`},
			expectedRules: []interface{}{CompPackage{"lalal", "yoyo"}, CompPackage{"toto"}},
		},
		"with a simple rule and contradictions": {
			jsonRules:     []string{`["lalal","yoyo","-yoyo"]`},
			expectedRules: []interface{}{CompPackage{"lalal"}},
		},
		"with a simple rule and contradictions in two different rules": {
			jsonRules:     []string{`["lalal","yoyo"]`, `["-yoyo"]`},
			expectedRules: []interface{}{CompPackage{"lalal"}, CompPackage{}},
		},
	}

	for name, c := range testCases {
		t.Run(name, func(t *testing.T) {
			obj := CompPackages{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
			for _, rule := range c.jsonRules {
				require.NoError(t, obj.Add(rule))
			}
			require.Equal(t, c.expectedRules, obj.rules)
			rulePackages = map[string]any{}
			blacklistedPackages = map[string]any{}
		})
	}
}
