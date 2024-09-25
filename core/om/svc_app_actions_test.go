package om

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/testhelper"
	"github.com/opensvc/om3/util/usergroup"
)

func TestAppStop(t *testing.T) {
	operationNotPermittedMsg := "operation not permitted"
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "solaris" {
		operationNotPermittedMsg = " not owner"
	}
	cases := map[string]struct {
		extraArgs       []string
		expectedResults []string
	}{
		"logInfo": {
			[]string{"--rid", "app#1", "--log=debug"},
			[]string{"app#1: stdout: line2"},
		},
		"logError": {
			[]string{"--rid", "app#2", "--log=debug"},
			[]string{"unrecognized option"},
		},
		"env": {
			[]string{"--rid", "app#env", "--log=debug"},
			[]string{"FOO=foo", "acceptMixedCase=value1"},
		},
		"cwd": {
			[]string{"--rid", "app#cwd", "--log=debug"},
			[]string{"/usr"},
		},
		"cwdWithDefaultType": {
			[]string{"--rid", "app#cwdWithDefaultType", "--log=debug"},
			[]string{"/usr"},
		},
		"badUser": {
			[]string{"--rid", "app#badUser", "--log=debug"},
			[]string{"unable to find user info for 'badUser'"},
		},
		"badGroup": {
			[]string{"--rid", "app#badGroup", "--log=debug"},
			[]string{"unable to find group info for 'badGroup'"},
		},
		"badUserGroup": {
			[]string{"--rid", "app#badUserGroup", "--log=debug"},
			[]string{"unable to find user info for 'badUser'"},
		},
		"root": {
			[]string{"--rid", "app#root", "--log=debug"},
			[]string{"uid=0(root) gid=1"}, // daemon may be 12 on solaris
		},
		"nonRoot": {
			[]string{"--rid", "app#root", "--log=debug"},
			[]string{operationNotPermittedMsg},
		},
		"stopTrueScript": {
			[]string{"--rid", "app#stopTrueScript", "--log=debug"},
			[]string{"noSuchFile.opensvc.test"},
		},
		"stoptrue": {
			[]string{"--rid", "app#stoptrue", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopTrue": {
			[]string{"--rid", "app#stopTrue", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopT": {
			[]string{"--rid", "app#stopT", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stop0": {
			[]string{"--rid", "app#stop0", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopf": {
			[]string{"--rid", "app#stopf", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopF": {
			[]string{"--rid", "app#stopF", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopfalse": {
			[]string{"--rid", "app#stopfalse", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopFALSE": {
			[]string{"--rid", "app#stopFALSE", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopFalse": {
			[]string{"--rid", "app#stopFalse", "--log=debug"},
			[]string{"stdout: stop"},
		},
		"stopEmpty": {
			extraArgs: []string{"--rid", "app#stopEmpty", "--log=debug"},
		},
		"stopUndef": {
			extraArgs: []string{"--rid", "app#stopUndef", "--log=debug"},
		},
		"stopScriptUndef": {
			[]string{"--rid", "app#stopScriptUndef", "--log=debug"},
			[]string{"action 'stop' as true value but 'script' keyword is empty"},
		},
		"configEnv": {
			[]string{"--rid", "app#configEnv", "--log=debug"},
			[]string{"FOOCFG1=fooValue1", "FooCFG2=fooValue2"},
		},
		"secretEnv": {
			[]string{"--rid", "app#secretEnv", "--log=debug"},
			[]string{"FOOSEC1=fooSec1", "FooSEC2=fooSec2"},
		},
		"secretEnvMatchers": {
			[]string{"--rid", "app#secretEnvMatchers", "--log=debug"},
			[]string{"foo.foo1=fooSec1", "foo.Foo2=fooSec2"},
		},
		"configEnvMatchers": {
			[]string{"--rid", "app#configEnvMatchers", "--log=debug"},
			[]string{"FOOKEY1=FOOKEYValue1", "FOOkey2=FOOkeyValue2"},
		},
	}

	getCmd := func(name string) []string {
		args := []string{"svcappforking", "stop", "--local", "--color", "no"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcappforking.conf", "etc/svcappforking.conf")
	env.InstallFile("../../testdata/cfg1_svcappforking.conf", "etc/cfg/svcappforking.conf")
	env.InstallFile("../../testdata/sec1_svcappforking.conf", "etc/sec/svcappforking.conf")

	t.Run("logInfo", func(t *testing.T) {
		name := "logInfo"
		var msg string
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		exitError, ok := err.(*exec.ExitError)
		if ok {
			msg = string(exitError.Stderr)
		} else {
			msg = ""
		}
		require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '%v'", string(out))
		}
	})

	t.Run("logError", func(t *testing.T) {
		name := "logError"
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, _ := cmd.CombinedOutput()
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '%v'", string(out))
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, expected) {
					assert.Containsf(t, line, "WRN", "stderr output line not logged with error level")
				}
			}
		}
	})

	t.Run("exit with error", func(t *testing.T) {
		name := "logError"
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		_, err := cmd.CombinedOutput()
		assert.NotNil(t, err)
	})

	t.Run("environment", func(t *testing.T) {
		name := "env"
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range cases[name].expectedResults {
			t.Run(strings.Split(expected, "=")[0], func(t *testing.T) {
				assert.Containsf(t, string(out), expected,
					"'%v' not found in out.\ngot:\n%v", expected, string(out))
			})
		}
		defaultEnv := []string{
			"OPENSVC_RID=app#env",
			"OPENSVC_NAME=svcappforking",
			"OPENSVC_KIND=svc",
			"OPENSVC_ID=f8fd968f-3dfd-4a54-a8c8-f5a52bbeb0c1",
			"OPENSVC_NAMESPACE=root",
		}
		for _, expected := range defaultEnv {
			t.Run("default:"+strings.Split(expected, "=")[0], func(t *testing.T) {
				assert.Containsf(t, string(out), expected,
					"'%v' not found in out.\ngot:\n%v", expected, string(out))
			})
		}
	})

	t.Run("default type is forking", func(t *testing.T) {
		name := "cwdWithDefaultType"
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("cwd", func(t *testing.T) {
		name := "cwd"
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, "got: %s", string(out))
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
		}
	})

	for _, name := range []string{"badUser", "badGroup", "badUserGroup"} {
		t.Run("invalid credentials "+name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			assert.NotNil(t, err, "got: '\n%v'", string(out))
			for _, expected := range cases[name].expectedResults {
				assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
			}
		})
	}

	t.Run("valid user and group", func(t *testing.T) {
		var name string
		if privUser, err := usergroup.IsPrivileged(); err != nil {
			t.Fail()
		} else if privUser {
			name = "root"
		} else {
			name = "nonRoot"
		}
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)

		if name == "root" {
			out, err := cmd.CombinedOutput()
			assert.Nil(t, err, "got: '\n%v'", string(out))
			for _, expected := range cases[name].expectedResults {
				assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
			}
		} else {
			out, err := cmd.CombinedOutput()
			assert.NotNil(t, err, "got: '\n%v'", string(out))
			for _, expected := range cases[name].expectedResults {
				assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
			}
		}
	})

	t.Run("when stop is true and script not found into <svcname>.d", func(t *testing.T) {
		name := "stopTrueScript"
		var msg string
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		exitError, ok := err.(*exec.ExitError)
		if ok {
			msg = string(exitError.Stderr)
		} else {
			msg = ""
		}
		require.NotNilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), env.Root+"/etc/svcappforking.d/"+expected+": no such file or directory", "got: '%v'", string(out))
		}
	})

	for _, name := range []string{"true", "True", "T"} {
		t.Run("when stop is true like ("+name+")", func(t *testing.T) {
			name := "stop" + name
			var msg string
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			exitError, ok := err.(*exec.ExitError)
			if ok {
				msg = string(exitError.Stderr)
			} else {
				msg = ""
			}
			require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
			for _, expected := range cases[name].expectedResults {
				assert.Containsf(t, string(out), expected, "got: '%v'", string(out))
			}
		})
	}

	for _, name := range []string{"0", "f", "F", "false", "FALSE", "False"} {
		t.Run("when stop is false like ("+name+")", func(t *testing.T) {
			name := "stop" + name
			var msg string
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			exitError, ok := err.(*exec.ExitError)
			if ok {
				msg = string(exitError.Stderr)
			} else {
				msg = ""
			}
			require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
			for _, expected := range cases[name].expectedResults {
				assert.NotContainsf(t, string(out), expected, "got: '%v'", string(out))
			}
		})
	}

	t.Run("when no command stop", func(t *testing.T) {
		for _, name := range []string{"stopEmpty", "stopUndef"} {
			t.Run(name, func(t *testing.T) {
				var msg string
				args := getCmd(name)
				t.Logf("run 'om %v'", strings.Join(args, " "))
				cmd := exec.Command(os.Args[0], args...)
				cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
				out, err := cmd.CombinedOutput()
				exitError, ok := err.(*exec.ExitError)
				if ok {
					msg = string(exitError.Stderr)
				} else {
					msg = ""
				}
				require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
				require.NotContains(t, string(out), "INF run ", "expected no run")
			})
		}
	})

	t.Run("stop value true without script keyword exit non 0", func(t *testing.T) {
		name := "stopScriptUndef"
		var msg string
		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		exitError, ok := err.(*exec.ExitError)
		if ok {
			msg = string(exitError.Stderr)
		} else {
			msg = ""
		}
		require.NotNilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '%v'", string(out))
		}
	})

	t.Run("configs_environment", func(t *testing.T) {
		name := "configEnv"

		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("secrets_environment", func(t *testing.T) {
		name := "secretEnv"

		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("secrets_environment_matcher", func(t *testing.T) {
		name := "secretEnvMatchers"

		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("config_environment_matcher", func(t *testing.T) {
		name := "configEnvMatchers"

		args := getCmd(name)
		t.Logf("run 'om %v'", strings.Join(args, " "))
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, "got '%v'", string(out))
		for _, expected := range cases[name].expectedResults {
			assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
		}
	})
}

func TestAppStopStartSequence(t *testing.T) {
	cases := map[string]struct {
		Action    string
		ExtraArgs []string
		Expected  []string
	}{
		// TODO
		//"start with mixed start sequence numbers and no sequence numbers": {
		//	[]string{},
		//	[]string{"rid1", "rid3", "rid2", "rid4", "rid5"},
		//  found:  {"rid5", "rid1", "rid2", "rid3", "rid4"}
		//},
		"stop with mixed start sequence numbers and no sequence numbers": {
			"stop",
			[]string{},
			[]string{"rid5", "rid4", "rid2", "rid3", "rid1"},
		},
		"stop when only start sequence numbers": {
			"stop",
			[]string{"--rid", "app#rid1,app#rid2,app#rid3"},
			[]string{"rid2", "rid3", "rid1"},
		},
		"start when only start sequence numbers": {
			"start",
			[]string{"--rid", "app#rid1,app#rid2,app#rid3"},
			[]string{"rid1", "rid3", "rid2"},
		},
		"stop when no start sequence numbers": {
			"stop",
			[]string{"--rid", "app#rid5,app#rid4"},
			[]string{"rid5", "rid4"},
		},
		"start when no start sequence numbers": {
			"start",
			[]string{"--rid", "app#rid5,app#rid4"},
			[]string{"rid4", "rid5"},
		},
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", cases[name].Action, "--log=info", "--color", "no", "--local"}
		args = append(args, cases[name].ExtraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcapp1.conf", "etc/svcapp.conf")

	for name := range cases {
		t.Run("orderBasedOnStartID:"+name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, "got '%v'", string(out))
			compile, err := regexp.Compile(": app#(rid[0-9]+) " + cases[name].Action)
			require.Nil(t, err)
			var foundSequence []string
			for _, match := range compile.FindAllStringSubmatch(string(out), -1) {
				foundSequence = append(foundSequence, match[1])
			}

			assert.Equalf(t, cases[name].Expected, foundSequence, "got:\n%v", string(out))
		})
	}
}

func TestAppStopComplexCommand(t *testing.T) {
	cases := map[string]struct {
		ExtraArgs   []string
		Expected    []string
		NotExpected []string
	}{
		"echoOneAndEchoTwo": {
			ExtraArgs: []string{"--rid", "app#echoOneAndEchoTwo"},
			Expected:  []string{"stdout: One", "stdout: Two"},
		},
		"echoOneOrEchoTwo": {
			ExtraArgs:   []string{"--rid", "app#echoOneOrEchoTwo"},
			Expected:    []string{"stdout: One"},
			NotExpected: []string{"stdout: Two"},
		},
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "stop", "--local", "--log=debug", "--color=no"}
		args = append(args, cases[name].ExtraArgs...)
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcappComplexCommand.conf", "etc/svcapp.conf")

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, "got '%v'", string(out))
			for _, expected := range test.Expected {
				assert.Containsf(t, string(out), expected, "got:\n%v", string(out))
			}
			for _, notExpected := range test.NotExpected {
				assert.NotContainsf(t, string(out), notExpected, "got:\n%v", string(out))
			}
		})
	}
}

func TestAppStopLimit(t *testing.T) {
	cases := map[string][]string{
		"limit_cpu": {"3602"},
		// TODO 50 on 2.1, vs 0 omg: "limit_core":    {"100"},
		"limit_data":    {"41943040"}, // TODO 40000000 on 2.1
		"limit_fsize":   {"512"},      // TODO 500 on 2.1
		"limit_memlock": {"63"},
		"limit_nofile":  {"128"},
		"limit_nproc":   {"200"},
		"limit_stack":   {"1024"}, // TODO 1000 on 2.1 Linux
		// TODO Document limit_vmem now supported on Linux (vs RLIMIT_VMEM error on 2.1)
		"limit_vmem":    {"41943040"},
		"limit_2_items": {"128", "63"},
	}
	skipGOOS := map[string][]string{
		"solaris": {"limit_memlock", "limit_nproc"},
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "stop", "--local", "--color=no", "--log=debug", "--rid", "app#" + name}
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcappforking_limit.conf", "etc/svcapp.conf")

	for name, expecteds := range cases {
		t.Run(name, func(t *testing.T) {
			if toSkip, ok := skipGOOS[runtime.GOOS]; ok {
				for _, testToSkipp := range toSkip {
					if name == testToSkipp {
						t.Skipf("skipped on %v", runtime.GOOS)
					}
				}
			}
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)

			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, "got '%v'", string(out))
			for _, expected := range expecteds {
				assert.Containsf(t, string(out), expected, "got:\n%v", string(out))
			}
		})
	}
}

func TestAppStopTimeout(t *testing.T) {
	cases := map[string]bool{
		"no_timeout":           true,
		"stop_timeout_succeed": true,
		"stop_timeout_failure": false,
		"timeout_failure":      false,
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "stop", "--local", "--rid", "app#" + name}
		return args
	}

	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	env.InstallFile("../../testdata/svcappforking_timeout.conf", "etc/svcapp.conf")

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			args := getCmd(name)
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			if cases[name] {
				require.Nilf(t, err, "expected succeed, got '%v'", string(out))
			} else {
				require.NotNil(t, err, "  expected failure, got '%v'", string(out))
			}
		})
	}
}

func TestAppStartRollback(t *testing.T) {
	cases := map[string]struct {
		rids               []string
		extraArgs          []string
		expectedStart      []string
		unexpectedStart    []string
		expectedRollback   []string
		unexpectedRollback []string
		exitCode           int
	}{
		"when all app succeed": {
			rids:               []string{"1ok", "2ok"},
			expectedStart:      []string{"1ok", "2ok"},
			unexpectedRollback: []string{"1ok", "2ok"},
		},
		"when one app fails": {
			rids: []string{"1ok", "2ok", "3fail", "6ok"},
			// start apps until one fails
			expectedStart: []string{"1ok", "2ok", "3fail"},
			// do not continue start after one app fails
			unexpectedStart: []string{"6ok"},
			// rollback is are only called on success started app
			// TODO verify expectedRollback: []string{"1ok", "2ok"},
			expectedRollback: []string{},
			// ensure app without succeed cmd start are not rollback
			unexpectedRollback: []string{"3fail", "6ok"},
			exitCode:           1,
		},
		"when one app fails but rollback is disabled": {
			rids:          []string{"1ok", "2ok", "3fail", "6ok"},
			extraArgs:     []string{"--disable-rollback"},
			expectedStart: []string{"1ok", "2ok", "3fail"},
			// do not continue start after one app fails
			unexpectedStart: []string{"6ok"},
			// no rollback because of --disable-rollback
			expectedRollback:   []string{},
			unexpectedRollback: []string{"1ok", "2ok", "3fail", "6ok"},
			exitCode:           1,
		},
		"do not continue rollbacks when one rollback step fails": {
			rids: []string{"1ok", "2ok", "4rollbackFail", "5fail", "6ok"},
			// start apps until one fails
			expectedStart: []string{"1ok", "2ok", "4rollbackFail", "5fail"},
			// do not continue start after one app fails
			unexpectedStart:  []string{"6ok"},
			expectedRollback: []string{"4rollbackFail"},
			// do not continue  rollbacks when one rollback fails
			unexpectedRollback: []string{"1ok", "2ok", "5fail", "6ok"},
			exitCode:           1,
		},
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "start", "--local", "--color", "no"}
		var rids []string
		for _, rid := range cases[name].rids {
			rids = append(rids, "app#"+rid)
		}
		if len(rids) > 0 {
			args = append(args, "--rid", strings.Join(rids, ","))
		}
		if len(cases[name].extraArgs) > 0 {
			args = append(args, cases[name].extraArgs...)
		}
		return args
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			env := testhelper.Setup(t)
			env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
			env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
			env.InstallFile("../../testdata/svcapp-rollback.conf", "etc/svcapp.conf")
			args := getCmd(name)
			args = append(args, "--log", "debug")
			t.Logf("run 'om %v'", strings.Join(args, " "))
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "GO_TEST_MODE=off", "OSVC_ROOT_PATH="+env.Root)
			out, err := cmd.CombinedOutput()
			t.Logf("output:\n%v", string(out))
			expectedExitCode := test.exitCode
			if expectedExitCode == 0 {
				t.Run("expected exit code 0", func(t *testing.T) {
					t.Logf("from 'om %v'", strings.Join(args, " "))
					require.NoErrorf(t, err, "unexpected exit code: %v", err)
					// Add delay for file system cache updated
					time.Sleep(50 * time.Millisecond)
				})
			} else {
				t.Run("expected exit code non 0", func(t *testing.T) {
					t.Logf("from 'om %v'", strings.Join(args, " "))
					require.IsTypef(t, &exec.ExitError{}, err, "unexpected error type %v", err)
					require.Equalf(
						t,
						expectedExitCode,
						err.(*exec.ExitError).ExitCode(),
						"unexpected exit code.\nout: '%v'",
						string(out))
				})
				// Add delay for file system cache updated
				time.Sleep(50 * time.Millisecond)
			}

			t.Run("expected start", func(t *testing.T) {
				t.Logf("from 'om %v'\nexpected starts: %v", strings.Join(args, " "), test.expectedStart)
				for _, rid := range test.expectedStart {
					trace := "app#" + rid + "-start.trace"
					assert.FileExistsf(
						t,
						filepath.Join(env.Root, "var", trace),
						"expected start not found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check start is called for rid %v", rid)
				}
			})

			t.Run("expected rollback", func(t *testing.T) {
				t.Logf("from 'om %v'\nexpected rollbacks: %v", strings.Join(args, " "), test.expectedRollback)
				for _, rid := range test.expectedRollback {
					trace := "app#" + rid + "-rollback.trace"
					assert.FileExistsf(
						t,
						filepath.Join(env.Root, "var", trace),
						"expected rollback not found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check rollback is called for rid %v", rid)
				}
			})

			t.Run("unexpected start", func(t *testing.T) {
				t.Logf("from 'om %v'\nunexpected starts: %v", strings.Join(args, " "), test.unexpectedStart)
				for _, rid := range test.unexpectedStart {
					trace := "app#" + rid + "-start.trace"
					assert.NoFileExists(
						t,
						filepath.Join(env.Root, "var", trace),
						"unexpected start found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check start cmd is not called for rid %v", rid)
				}
			})

			t.Run("unexpected rollback", func(t *testing.T) {
				t.Logf("from 'om %v'\nunexpected rollback: %v", strings.Join(args, " "), test.unexpectedRollback)
				for _, rid := range test.unexpectedRollback {
					trace := "app#" + rid + "-rollback.trace"
					assert.NoFileExists(
						t,
						filepath.Join(env.Root, "var", trace),
						"unexpected rollback found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check rollback is not called for rid %v", rid)
				}
			})
		})
	}
}
