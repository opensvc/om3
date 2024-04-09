package command

import (
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/util/plog"
)

func TestString(t *testing.T) {
	cases := []struct {
		Name     string
		Args     []string
		Expected string
	}{
		{
			Name:     "",
			Args:     nil,
			Expected: "",
		},
		{
			Name:     "/bin/ls",
			Args:     nil,
			Expected: "/bin/ls",
		},
		{
			Name:     "/bin/ls",
			Args:     []string{"foo", "bar"},
			Expected: "/bin/ls foo bar",
		},
		{
			Name:     "/bin/ls",
			Args:     []string{"foo bar"},
			Expected: "/bin/ls 'foo bar'",
		},
		{
			Name:     "/bin/bash",
			Args:     []string{"-c", "test `date +%Y` -eq 2020 && echo so dated"},
			Expected: "/bin/bash -c 'test `date +%Y` -eq 2020 && echo so dated'",
		},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s %q", c.Name, c.Args), func(t *testing.T) {
			cmd := T{name: c.Name, args: c.Args}
			assert.Equal(t, c.Expected, cmd.String())
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Run("Update SysProcAttr.credential from user and group", func(t *testing.T) {
		gid := uint32(1)
		if runtime.GOOS == "solaris" {
			gid = 12
		}
		cmd := T{
			cmd:   &exec.Cmd{},
			user:  "root",
			group: "daemon",
		}
		require.Nil(t, cmd.update())
		assert.Equalf(t, uint32(0), cmd.cmd.SysProcAttr.Credential.Uid, "invalid Uid")
		assert.Equalf(t, gid, cmd.cmd.SysProcAttr.Credential.Gid, "invalid Gid")
	})

	t.Run("Preserve existing SysProcAttr attr", func(t *testing.T) {
		cmd := exec.Cmd{}
		cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: "/tmp"}
		xCmd := T{
			cmd:  &cmd,
			user: "root",
		}
		require.Nil(t, xCmd.update())
		assert.Equalf(t, "/tmp", xCmd.cmd.SysProcAttr.Chroot, "unexpected change")
	})
}

func TestNew(t *testing.T) {
	t.Run("WithLogger", func(t *testing.T) {
		prefix := "test"
		log := plog.NewDefaultLogger().Attr("pkg", "util/command").WithPrefix(prefix)
		c := New(WithLogger(log))
		assert.Equal(t, prefix, c.log.Prefix())
	})
}

func TestT_StdoutStderr(t *testing.T) {
	cases := map[string]struct {
		name   string
		args   []string
		stdout []byte
		stderr []byte
	}{
		"withOnlyStdout": {
			name:   "bash",
			args:   []string{"-c", "echo foo; echo bar"},
			stdout: []byte("foo\nbar"),
			stderr: nil,
		},
		"withWithEmptyLine": {
			name:   "bash",
			args:   []string{"-c", "echo; echo >&2"},
			stdout: []byte("\n"),
			stderr: []byte("\n"),
		},
		"withOnlyStderr": {
			name:   "bash",
			args:   []string{"-c", "echo foo >&2; echo bar >&2"},
			stdout: nil,
			stderr: []byte("foo\nbar"),
		},
		"withStdoutAndStderr": {
			name:   "bash",
			args:   []string{"-c", "echo foo >&2; echo bar"},
			stdout: []byte("bar"),
			stderr: []byte("foo"),
		},
		"withNoStdoutAndStderr": {
			name:   "bash",
			args:   []string{"-c", "true"},
			stdout: nil,
			stderr: nil,
		},
	}
	for name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Logf("call %s %q", cases[name].name, cases[name].args)
			cmd := New(WithName(cases[name].name), WithVarArgs(cases[name].args...), WithBufferedStdout(), WithBufferedStderr())
			assert.Nil(t, cmd.Run())
			t.Run("has expected stdout", func(t *testing.T) {
				got := string(cmd.Stdout())
				expected := string(cases[name].stdout)
				assert.Equalf(t, expected, got, "got '%v' instead of '%v'", got, expected)
			})
			t.Run("has expected stderr", func(t *testing.T) {
				got := cmd.Stderr()
				expected := cases[name].stderr
				assert.Equalf(t, expected, got, "got '%v' instead of '%v'", got, expected)
			})
		})
	}
}

func TestStart(t *testing.T) {
	t.Run("can not call Start twice", func(t *testing.T) {
		cmd := New(WithName("pwd"))
		assert.Nil(t, cmd.Start())
		assert.Error(t, ErrAlreadyStarted, cmd.Start())
	})
}

func TestRun(t *testing.T) {
	t.Run("can not call Run twice", func(t *testing.T) {
		cmd := New(WithName("pwd"))
		assert.Nil(t, cmd.Run())
		assert.Error(t, ErrAlreadyStarted, cmd.Run())
	})
}

func TestWait(t *testing.T) {
	t.Run("can not call Wait twice", func(t *testing.T) {
		cmd := New(WithName("pwd"))
		assert.Nil(t, cmd.Start())
		assert.Nil(t, cmd.Wait())
		assert.Equal(t, ErrAlreadyWaited, cmd.Wait())
	})

	t.Run("return nil and has correct exit code when exit code in WithIgnoredExitCodes", func(t *testing.T) {
		t.Run("Without funcopt WithIgnoredExitCodes()", func(t *testing.T) {
			cmd := New(WithName("pwd"))
			assert.Nil(t, cmd.Start())
			assert.Nil(t, cmd.Wait())
			assert.Equal(t, 0, cmd.ExitCode())
		})
		t.Run("exit 2 when WithIgnoredExitCodes(2)", func(t *testing.T) {
			cmd := New(WithName("bash"), WithVarArgs("-c", "exit 2"), WithIgnoredExitCodes(2))
			assert.Nil(t, cmd.Start())
			assert.Nil(t, cmd.Wait())
			assert.Equal(t, 2, cmd.ExitCode())
		})
		t.Run("exit 3 when WithIgnoredExitCodes(2, 3)", func(t *testing.T) {
			cmd := New(WithName("bash"), WithVarArgs("-c", "exit 3"), WithIgnoredExitCodes(2, 3))
			assert.Nil(t, cmd.Start())
			assert.Nil(t, cmd.Wait())
			assert.Equal(t, 3, cmd.ExitCode())
		})
		t.Run("exit 0 when WithIgnoredExitCodes(0, 2, 3)", func(t *testing.T) {
			cmd := New(WithName("bash"), WithVarArgs("-c", "exit 0"), WithIgnoredExitCodes(0, 2, 3))
			assert.Nil(t, cmd.Start())
			assert.Nil(t, cmd.Wait())
			assert.Equal(t, 0, cmd.ExitCode())
		})
		t.Run("exit 66 when WithIgnoredExitCodes() ignore all exit codes", func(t *testing.T) {
			cmd := New(WithName("bash"), WithVarArgs("-c", "exit 66"), WithIgnoredExitCodes())
			assert.Nil(t, cmd.Start())
			assert.Nil(t, cmd.Wait())
			assert.Equal(t, 66, cmd.ExitCode())
		})
	})

	t.Run("exit 0 when WithIgnoredExitCodes(3, 4) return ErrExitCode", func(t *testing.T) {
		type unwrapper interface {
			Unwrap() error
		}
		cmd := New(WithName("bash"), WithVarArgs("-c", "exit 0"), WithIgnoredExitCodes(3, 4))
		assert.Nil(t, cmd.Start())
		err := cmd.Wait()
		assert.NotNil(t, err)
		assert.IsType(t, &ErrExitCode{}, err.(unwrapper).Unwrap())
		assert.Error(t, err)
		assert.ErrorContains(t, err, "exit code 0 not in success codes: [3 4]")
		assert.Equal(t, 0, cmd.ExitCode())
	})
}
