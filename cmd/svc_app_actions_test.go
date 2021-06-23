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
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/usergroup"
)

func TestAppStop(t *testing.T) {
	operation_not_permitted_msg := "operation not permitted"
	if runtime.GOOS == "solaris" {
		operation_not_permitted_msg = " not owner"
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
			"/bin/ls: ",
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
		"baduser": {
			[]string{"--rid", "app#baduser"},
			"unable to find user info for 'baduser'",
		},
		"badgroup": {
			[]string{"--rid", "app#badgroup"},
			"unable to find group info for 'badgroup'",
		},
		"badusergroup": {
			[]string{"--rid", "app#badusergroup"},
			"unable to find user info for 'baduser'",
		},
		"root": {
			[]string{"--rid", "app#root"},
			"uid=0(root) gid=1", // daemon may be 12 on solaris
		},
		"nonRoot": {
			[]string{"--rid", "app#root"},
			operation_not_permitted_msg,
		},
		"stoptruescriptd": {
			[]string{"--rid", "app#stoptruescriptd"},
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

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			d, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			td = d
		}
		test_conf_helper.InstallSvcFile(t, "cluster.conf", filepath.Join(td, "etc", "cluster.conf"))
		test_conf_helper.InstallSvcFile(t, "svcappforking.conf", filepath.Join(td, "etc", "svcappforking.conf"))
		test_conf_helper.InstallSvcFile(t, "cfg1_svcappforking.conf", filepath.Join(td, "etc", "cfg", "svcappforking.conf"))
		test_conf_helper.InstallSvcFile(t, "sec1_svcappforking.conf", filepath.Join(td, "etc", "sec", "svcappforking.conf"))

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd(name))
		return
	}

	t.Run("logInfo", func(t *testing.T) {
		name := "logInfo"
		var msg string
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
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
		name := "logError"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		out, _ := cmd.CombinedOutput()
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "err=\""+expected, "got: '%v'", string(out))
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "out="+expected) {
					assert.Containsf(t, line, "ERR", "stderr output line not logged with error level")
				}
			}
		}
	})

	t.Run("exit with error", func(t *testing.T) {
		name := "logError"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		_, err := cmd.CombinedOutput()
		assert.NotNil(t, err)
	})

	t.Run("environment", func(t *testing.T) {
		name := "env"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("default type is forking", func(t *testing.T) {
		name := "cwdWithDefaultType"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("cwd", func(t *testing.T) {
		name := "cwd"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		out, err := cmd.CombinedOutput()
		require.Nilf(t, err, "got: %s", string(out))
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "out="+expected, "got: '\n%v'", string(out))
		}
	})

	for _, name := range []string{"baduser", "badgroup", "badusergroup"} {
		t.Run("invalid credentials "+name, func(t *testing.T) {
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
			out, err := cmd.CombinedOutput()
			assert.NotNil(t, err, "got: '\n%v'", string(out))
			for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
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
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)

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
		name := "stoptruescriptd"
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
			name := "stop" + name
			var msg string
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
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
			name := "stop" + name
			var msg string
			t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
			cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
			cmd.Env = append(os.Environ(), "TC_NAME="+name)
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
				var msg string
				t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
				cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
				cmd.Env = append(os.Environ(), "TC_NAME="+name)
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
		name := "stopScriptUndef"
		var msg string
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
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

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			d, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			td = d
		}

		test_conf_helper.InstallSvcFile(t, "svcapp1.conf", filepath.Join(td, "etc", "svcapp.conf"))

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd(name))
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
			compile, err := regexp.Compile("running .*rid=app#([a-z0-9]+) ")
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

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			d, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			td = d
		}

		test_conf_helper.InstallSvcFile(t, "svcappComplexCommand.conf", filepath.Join(td, "etc", "svcapp.conf"))

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd(name))
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

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			d, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			td = d
		}

		test_conf_helper.InstallSvcFile(t, "svcappforking_limit.conf", filepath.Join(td, "etc", "svcapp.conf"))

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd(name))
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

	if name, ok := os.LookupEnv("TC_NAME"); ok == true {
		var td string
		if td, ok = os.LookupEnv("TC_PATHSVC"); ok != true {
			d, cleanup := testhelper.Tempdir(t)
			defer cleanup()
			td = d
		}

		test_conf_helper.InstallSvcFile(t, "svcappforking_timeout.conf", filepath.Join(td, "etc", "svcapp.conf"))

		rawconfig.Load(map[string]string{"osvc_root_path": td})
		defer rawconfig.Load(map[string]string{})
		defer hostname.Impersonate("node1")()
		ExecuteArgs(getCmd(name))
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
