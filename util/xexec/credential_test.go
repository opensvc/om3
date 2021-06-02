package xexec

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

func TestSetCredential(t *testing.T) {
	cases := []struct {
		user  string
		group string
	}{
		{"WrongUserX", "WrongGroupY"},
		{"WrongUserX", ""},
		{"", "WrongGroupY"},
	}
	for _, tc := range cases {
		name := "user: '" + tc.user + "' group '" + tc.group + "'"
		t.Run("return error for "+name, func(t *testing.T) {
			assert.NotNil(t, SetCredential(&exec.Cmd{}, tc.user, tc.group))
		})
		t.Run("does not update cmd.SysProcAttr for "+name, func(t *testing.T) {
			cmd := exec.Cmd{}
			_ = SetCredential(&cmd, tc.user, tc.group)
			assert.Nil(t, cmd.SysProcAttr)
		})
	}

	t.Run("Update SysProcAttr.Credential from user and group", func(t *testing.T) {
		cmd := exec.Cmd{}
		gid := uint32(1)
		if runtime.GOOS == "solaris" {
			gid = 12
		}
		require.Nil(t, SetCredential(&cmd, "root", "daemon"))
		assert.Equalf(t, uint32(0), cmd.SysProcAttr.Credential.Uid, "invalid Uid")
		assert.Equalf(t, gid, cmd.SysProcAttr.Credential.Gid, "invalid Gid")
	})

	t.Run("Preserve existing SysProcAttr attr", func(t *testing.T) {
		cmd := exec.Cmd{}
		cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: "/tmp"}
		require.Nil(t, SetCredential(&cmd, "root", ""))
		assert.Equalf(t, "/tmp", cmd.SysProcAttr.Chroot, "unexpected change")
	})
}
