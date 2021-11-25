package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/util/usergroup"
)

func TestAppStop(t *testing.T) {
	operationNotPermittedMsg := "operation not permitted"
	if runtime.GOOS == "solaris" {
		operationNotPermittedMsg = " not owner"
	}
	cases := map[string]struct {
		extraArgs       []string
		expectedResults string
	}{
		"logInfo": {
			[]string{"--rid", "app#1"},
			"line1\nline2",
		},
		"logError": {
			[]string{"--rid", "app#2"},
			"unrecognized option",
		},
		"env": {
			[]string{"--rid", "app#env"},
			"FOO=foo\nacceptMixedCase=value1",
		},
		"cwd": {
			[]string{"--rid", "app#cwd"},
			"/usr",
		},
		"cwdWithDefaultType": {
			[]string{"--rid", "app#cwdWithDefaultType"},
			"/usr",
		},
		"badUser": {
			[]string{"--rid", "app#badUser"},
			"unable to find user info for 'badUser'",
		},
		"badGroup": {
			[]string{"--rid", "app#badGroup"},
			"unable to find group info for 'badGroup'",
		},
		"badUserGroup": {
			[]string{"--rid", "app#badUserGroup"},
			"unable to find user info for 'badUser'",
		},
		"root": {
			[]string{"--rid", "app#root"},
			"uid=0(root) gid=1", // daemon may be 12 on solaris
		},
		"nonRoot": {
			[]string{"--rid", "app#root"},
			operationNotPermittedMsg,
		},
		"stopTrueScript": {
			[]string{"--rid", "app#stopTrueScript"},
			"noSuchFile.opensvc.test",
		},
		"stoptrue": {
			[]string{"--rid", "app#stoptrue"},
			"stop",
		},
		"stopTrue": {
			[]string{"--rid", "app#stopTrue"},
			"stop",
		},
		"stopT": {
			[]string{"--rid", "app#stopT"},
			"stop",
		},
		"stop0": {
			[]string{"--rid", "app#stop0"},
			"stop",
		},
		"stopf": {
			[]string{"--rid", "app#stopf"},
			"stop",
		},
		"stopF": {
			[]string{"--rid", "app#stopF"},
			"stop",
		},
		"stopfalse": {
			[]string{"--rid", "app#stopfalse"},
			"stop",
		},
		"stopFALSE": {
			[]string{"--rid", "app#stopFALSE"},
			"stop",
		},
		"stopFalse": {
			[]string{"--rid", "app#stopFalse"},
			"stop",
		},
		"stopEmpty": {
			extraArgs: []string{"--rid", "app#stopEmpty"},
		},
		"stopUndef": {
			extraArgs: []string{"--rid", "app#stopUndef"},
		},
		"stopScriptUndef": {
			[]string{"--rid", "app#stopScriptUndef"},
			"action 'stop' as true value but 'script' keyword is empty",
		},
		"configEnv": {
			[]string{"--rid", "app#configEnv"},
			"FOOCFG1=fooValue1\nFooCFG2=fooValue2\n",
		},
		"secretEnv": {
			[]string{"--rid", "app#secretEnv"},
			"FOOSEC1=fooSec1\nFooSEC2=fooSec2\n",
		},
		"secretEnvMatchers": {
			[]string{"--rid", "app#secretEnvMatchers"},
			"foo.foo1=fooSec1\nfoo.Foo2=fooSec2\n",
		},
		"configEnvMatchers": {
			[]string{"--rid", "app#configEnvMatchers"},
			"FOOKEY1=FOOKEYValue1\nFOOkey2=FOOkeyValue2\n",
		},
	}

	getCmd := func(name string) []string {
		args := []string{"svcappforking", "stop", "--local", "--colorlog", "no"}
		args = append(args, cases[name].extraArgs...)
		return args
	}

	confs := []configs{
		{"cluster.conf", "cluster.conf"},
		{"svcappforking.conf", "svcappforking.conf"},
		{"cfg1_svcappforking.conf", "cfg/svcappforking.conf"},
		{"sec1_svcappforking.conf", "sec/svcappforking.conf"},
	}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	t.Run("logInfo", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "logInfo"
		var msg string
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		exitError, ok := err.(*exec.ExitError)
		if ok {
			msg = string(exitError.Stderr)
		} else {
			msg = ""
		}
		require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '%v'", string(out))
		}
	})

	t.Run("logError", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "logError"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, _ := cmd.CombinedOutput()
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), expected, "got: '%v'", string(out))
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "out="+expected) {
					assert.Containsf(t, line, "ERR", "stderr output line not logged with error level")
				}
			}
		}
	})

	t.Run("exit with error", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "logError"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		_, err := cmd.CombinedOutput()
		assert.NotNil(t, err)
	})

	t.Run("environment", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "env"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			t.Run(strings.Split(expected, "=")[0], func(t *testing.T) {
				assert.Containsf(t, string(out), "out="+expected,
					"'%v' not found in out.\ngot:\n%v", "out="+expected, string(out))
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
				assert.Containsf(t, string(out), "out="+expected,
					"'%v' not found in out.\ngot:\n%v", "out="+expected, string(out))
			})
		}
	})

	t.Run("default type is forking", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "cwdWithDefaultType"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("cwd", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "cwd"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, "got: %s", string(out))
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	for _, name := range []string{"badUser", "badGroup", "badUserGroup"} {
		t.Run("invalid credentials "+name, func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			assert.NotNil(t, err, "got: '\n%v'", string(out))
			for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
				assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
			}
		})
	}

	t.Run("valid user and group", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		var name string
		if privUser, err := usergroup.IsPrivileged(); err != nil {
			t.Fail()
		} else if privUser {
			name = "root"
		} else {
			name = "nonRoot"
		}
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)

		if name == "root" {
			out, err := cmd.CombinedOutput()
			assert.Nil(t, err, "got: '\n%v'", string(out))
			for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
				assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
			}
		} else {
			out, err := cmd.CombinedOutput()
			assert.NotNil(t, err, "got: '\n%v'", string(out))
			for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
				assert.Containsf(t, string(out), expected, "got: '\n%v'", string(out))
			}
		}
	})

	t.Run("when stop is true and script not found into <svcname>.d", func(t *testing.T) {
		name := "stopTrueScript"
		var msg string
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		exitError, ok := err.(*exec.ExitError)
		if ok {
			msg = string(exitError.Stderr)
		} else {
			msg = ""
		}
		require.NotNilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), td+"/etc/svcappforking.d/"+expected+": no such file or directory", "got: '%v'", string(out))
		}
	})

	for _, name := range []string{"true", "True", "T"} {
		t.Run("when stop is true like ("+name+")", func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			name := "stop" + name
			var msg string
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			exitError, ok := err.(*exec.ExitError)
			if ok {
				msg = string(exitError.Stderr)
			} else {
				msg = ""
			}
			require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
			for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
				assert.Containsf(t, string(out), "out="+expected, "got: '%v'", string(out))
			}
		})
	}

	for _, name := range []string{"0", "f", "F", "false", "FALSE", "False"} {
		t.Run("when stop is false like ("+name+")", func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			name := "stop" + name
			var msg string
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			exitError, ok := err.(*exec.ExitError)
			if ok {
				msg = string(exitError.Stderr)
			} else {
				msg = ""
			}
			require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
			for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
				assert.NotContainsf(t, string(out), "out="+expected, "got: '%v'", string(out))
			}
		})
	}

	t.Run("when no command stop", func(t *testing.T) {
		for _, name := range []string{"stopEmpty", "stopUndef"} {
			t.Run(name, func(t *testing.T) {
				td, cleanup := testhelper.Tempdir(t)
				defer cleanup()
				var msg string
				t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
				cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
				cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
				out, err := cmd.CombinedOutput()
				exitError, ok := err.(*exec.ExitError)
				if ok {
					msg = string(exitError.Stderr)
				} else {
					msg = ""
				}
				require.Nilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
				require.NotContains(t, string(out), "running", "expected no running")
			})
		}
	})

	t.Run("stop value true without script keyword exit non 0", func(t *testing.T) {
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()
		name := "stopScriptUndef"
		var msg string
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		exitError, ok := err.(*exec.ExitError)
		if ok {
			msg = string(exitError.Stderr)
		} else {
			msg = ""
		}
		require.NotNilf(t, err, "err: '%v', stderr: '%v', out='%v'", err, msg, string(out))
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), expected, "got: '%v'", string(out))
		}
	})

	t.Run("configs_environment", func(t *testing.T) {
		name := "configEnv"
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()

		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("secrets_environment", func(t *testing.T) {
		name := "secretEnv"
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()

		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("secrets_environment_matcher", func(t *testing.T) {
		name := "secretEnvMatchers"
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()

		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("config_environment_matcher", func(t *testing.T) {
		name := "configEnvMatchers"
		td, cleanup := testhelper.Tempdir(t)
		defer cleanup()

		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, "got '%v'", string(out))
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})
}

func TestAppStopStartSequence(t *testing.T) {
	cases := map[string]struct {
		ExtraArgs []string
		Expected  []string
	}{
		"start with mixed start sequence numbers and no sequence numbers": {
			[]string{},
			[]string{"rid1", "rid3", "rid2", "rid4", "rid5"},
		},
		"stop with mixed start sequence numbers and no sequence numbers": {
			[]string{},
			[]string{"rid5", "rid4", "rid2", "rid3", "rid1"},
		},
		"stop when only start sequence numbers": {
			[]string{"--rid", "app#rid1,app#rid2,app#rid3"},
			[]string{"rid2", "rid3", "rid1"},
		},
		"start when only start sequence numbers": {
			[]string{"--rid", "app#rid1,app#rid2,app#rid3"},
			[]string{"rid1", "rid3", "rid2"},
		},
		"stop when no start sequence numbers": {
			[]string{"--rid", "app#rid5,app#rid4"},
			[]string{"rid5", "rid4"},
		},
		"start when no start sequence numbers": {
			[]string{"--rid", "app#rid5,app#rid4"},
			[]string{"rid4", "rid5"},
		},
	}
	getCmd := func(name string) []string {
		var action string
		if strings.HasPrefix(name, "start") {
			action = "start"
		} else {
			action = "stop"
		}
		args := []string{"svcapp", action, "--colorlog", "no", "--local"}
		args = append(args, cases[name].ExtraArgs...)
		return args
	}

	confs := []configs{{"svcapp1.conf", "svcapp.conf"}}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	for name := range cases {
		t.Run("orderBasedOnStartId:"+name, func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStopStartSequence")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, "got '%v'", string(out))
			compile, err := regexp.Compile("out=.app#([a-z0-9]+) ")
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
			Expected:  []string{"One", "Two"},
		},
		"echoOneOrEchoTwo": {
			ExtraArgs:   []string{"--rid", "app#echoOneOrEchoTwo"},
			Expected:    []string{"One"},
			NotExpected: []string{"Two"},
		},
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "stop", "--local", "--colorlog=no"}
		args = append(args, cases[name].ExtraArgs...)
		return args
	}

	confs := []configs{{"svcappComplexCommand.conf", "svcapp.conf"}}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStopComplexCommand")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, "got '%v'", string(out))
			for _, expected := range cases[name].Expected {
				assert.Containsf(t, string(out), "out="+expected, "got:\n%v", string(out))
			}
			for _, notExpected := range cases[name].NotExpected {
				assert.NotContainsf(t, string(out), "out="+notExpected, "got:\n%v", string(out))
			}
		})
	}
}

func TestAppStopLimit(t *testing.T) {
	cases := map[string][]string{
		"limit_cpu":     {"3602"},
		"limit_core":    {"100"},
		"limit_data":    {"4000"},
		"limit_fsize":   {"1000"},
		"limit_memlock": {"63"},
		"limit_nofile":  {"128"},
		"limit_nproc":   {"200"},
		"limit_stack":   {"100"},
		"limit_vmem":    {"3000"},
		"limit_2_items": {"129", "4000"},
	}
	skipGOOS := map[string][]string{
		"solaris": {"limit_memlock", "limit_nproc"},
	}
	getCmd := func(name string) []string {
		args := []string{"svcapp", "stop", "--local", "--colorlog=no", "--rid", "app#" + name}
		return args
	}

	confs := []configs{{"svcappforking_limit.conf", "svcapp.conf"}}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			if toSkip, ok := skipGOOS[runtime.GOOS]; ok {
				for _, testToSkipp := range toSkip {
					if name == testToSkipp {
						t.Skipf("skipped on %v", runtime.GOOS)
					}
				}
			}
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStopLimit")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)

			out, err := cmd.CombinedOutput()
			require.Nilf(t, err, "got '%v'", string(out))
			for _, expected := range cases[name] {
				assert.Containsf(t, string(out), "out="+expected, "got:\n%v", string(out))
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

	confs := []configs{{"svcappforking_timeout.conf", "svcapp.conf"}}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStopTimeout")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
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
			expectedRollback: []string{"1ok", "2ok"},
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
		args := []string{"svcapp", "start", "--local", "--colorlog", "no"}
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

	confs := []configs{{"svcapp-rollback.conf", "svcapp.conf"}}
	if executeArgsTest(t, getCmd, confs) {
		return
	}

	for name := range cases {
		t.Run(name, func(t *testing.T) {
			td, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStartRollback")
			cmd.Env = append(os.Environ(), "TC_NAME="+name, "TC_PATHSVC="+td)
			out, err := cmd.CombinedOutput()
			t.Logf("output:\n%v", string(out))
			expectedExitCode := cases[name].exitCode
			if expectedExitCode == 0 {
				t.Run("expected exit code 0", func(t *testing.T) {
					t.Logf("from 'om %v'", strings.Join(getCmd(name), " "))
					require.Nilf(t, err, "unexpected exit code: %v, out: '%v'", err, string(out))
				})
			} else {
				t.Run("expected exit code non 0", func(t *testing.T) {
					t.Logf("from 'om %v'", strings.Join(getCmd(name), " "))
					require.IsTypef(t, &exec.ExitError{}, err, "unexpected error type %v, out: '%v'", err, string(out))
					require.Equalf(
						t,
						expectedExitCode,
						err.(*exec.ExitError).ExitCode(),
						"unexpected exit code.\nout: '%v'",
						string(out))
				})
			}

			expectedStart := cases[name].expectedStart
			t.Run("expected start", func(t *testing.T) {
				t.Logf("from 'om %v'\nexpected starts: %v", strings.Join(getCmd(name), " "), expectedStart)
				for _, rid := range expectedStart {
					trace := "app#" + rid + "-start.trace"
					assert.FileExistsf(
						t,
						filepath.Join(td, "var", trace),
						"expected start not found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check start is called for rid %v", rid)
				}
			})

			expectedRollback := cases[name].expectedRollback
			t.Run("expected rollback", func(t *testing.T) {
				t.Logf("from 'om %v'\nexpected rollbacks: %v", strings.Join(getCmd(name), " "), expectedRollback)
				for _, rid := range expectedRollback {
					trace := "app#" + rid + "-rollback.trace"
					assert.FileExistsf(
						t,
						filepath.Join(td, "var", trace),
						"expected rollback not found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check rollback is called for rid %v", rid)
				}
			})

			unexpectedStart := cases[name].unexpectedStart
			t.Run("unexpected start", func(t *testing.T) {
				t.Logf("from 'om %v'\nunexpected starts: %v", strings.Join(getCmd(name), " "), unexpectedStart)
				for _, rid := range unexpectedStart {
					trace := "app#" + rid + "-start.trace"
					assert.NoFileExists(
						t,
						filepath.Join(td, "var", trace),
						"unexpected start found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check start cmd is not called for rid %v", rid)
				}
			})

			unexpectedRollback := cases[name].unexpectedRollback
			t.Run("unexpected rollback", func(t *testing.T) {
				t.Logf("from 'om %v'\nunexpected rollback: %v", strings.Join(getCmd(name), " "), unexpectedRollback)
				for _, rid := range unexpectedRollback {
					trace := "app#" + rid + "-rollback.trace"
					assert.NoFileExists(
						t,
						filepath.Join(td, "var", trace),
						"unexpected rollback found: %v\nout:'%v'",
						rid,
						string(out))
					t.Logf("check rollback is not called for rid %v", rid)
				}
			})
		})
	}
}
