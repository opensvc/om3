package cmd

import (
	"github.com/opensvc/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/test_conf_helper"
	"opensvc.com/opensvc/util/usergroup"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppStop(t *testing.T) {
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
			"FOO=foo\nBAR=bar",
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
			"unable to set credential from user 'baduser'",
		},
		"badgroup": {
			[]string{"--rid", "app#badgroup"},
			"unable to set credential from user '', group 'badgroup'",
		},
		"badusergroup": {
			[]string{"--rid", "app#badusergroup"},
			"unable to set credential from user 'baduser', group 'badgroup'\n" +
				"unable to find user info for 'baduser'",
		},
		"root": {
			[]string{"--rid", "app#root"},
			"uid=0(root) gid=1(daemon)",
		},
		"nonRoot": {
			[]string{"--rid", "app#root"},
			"operation not permitted",
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
	}

	getCmd := func(name string) []string {
		args := []string{"svc", "-s", "svcappforking", "stop", "--color", "no", "--local"}
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
		test_conf_helper.InstallSvcFile(t, "svcappforking.conf", filepath.Join(td, "etc", "svcappforking.conf"))

		config.Load(map[string]string{"osvc_root_path": td})
		defer config.Load(map[string]string{})
		origHostname := config.Node.Hostname
		config.Node.Hostname = "node1"
		defer func() { config.Node.Hostname = origHostname }()
		config.Node.Hostname = "node1"
		rootCmd.SetArgs(getCmd(name))
		err := rootCmd.Execute()
		require.Nil(t, err)
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
			assert.Containsf(t, string(out), "| "+expected, "got: '%v'", string(out))
		}
	})

	t.Run("logError", func(t *testing.T) {
		name := "logError"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		out, _ := cmd.CombinedOutput()
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "| "+expected, "got: '%v'", string(out))
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "| "+expected) {
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
			assert.Containsf(t, string(out), "| "+expected, "got: '\n%v'", string(out))
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
			assert.Containsf(t, string(out), "| "+expected, "got: '\n%v'", string(out))
		}
	})

	t.Run("cwd", func(t *testing.T) {
		name := "cwd"
		t.Logf("run 'om %v'", strings.Join(getCmd(name), " "))
		cmd := exec.Command(os.Args[0], "-test.run=TestAppStop")
		cmd.Env = append(os.Environ(), "TC_NAME="+name)
		out, err := cmd.CombinedOutput()
		require.Nil(t, err)
		for _, expected := range strings.Split(cases[name].expectedResults, "\n") {
			assert.Containsf(t, string(out), "| "+expected, "got: '\n%v'", string(out))
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
				assert.Containsf(t, string(out), "| "+expected, "got: '%v'", string(out))
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
				assert.NotContainsf(t, string(out), "| "+expected, "got: '%v'", string(out))
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
				require.Len(t, out, 0, "expected empty output")
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
}
