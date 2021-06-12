package command

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
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
			Name:     "/bin/true",
			Args:     nil,
			Expected: "/bin/true",
		},
		{
			Name:     "/bin/ls",
			Args:     []string{"foo", "bar"},
			Expected: "/bin/ls \"foo\" \"bar\"",
		},
		{
			Name:     "/bin/ls",
			Args:     []string{"foo bar"},
			Expected: "/bin/ls \"foo bar\"",
		},
		{
			Name:     "/bin/echo",
			Args:     []string{"date:", "$(date)"},
			Expected: "/bin/echo \"date:\" \"$(date)\"",
		},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s %q", c.Name, c.Args), func(t *testing.T) {
			cmd := T{name: c.Name, args: c.Args}
			assert.Equal(t, c.Expected, cmd.String())
		})
	}
}

func Test_update(t *testing.T) {
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
		log := zerolog.Logger{}
		c := New(WithLogger(&log))
		assert.Equal(t, &log, c.log)
	})
}
