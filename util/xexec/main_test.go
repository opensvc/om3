package xexec

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
)

func TestT_Update(t *testing.T) {
	t.Run("Update SysProcAttr.Credential from user and group", func(t *testing.T) {
		cmd := exec.Cmd{}
		gid := uint32(1)
		if runtime.GOOS == "solaris" {
			gid = 12
		}
		xCmd := T{}
		cred, err := Credential("root", "daemon")
		require.Nil(t, err)
		xCmd.Credential = cred
		require.Nil(t, xCmd.Update(&cmd))
		assert.Equalf(t, uint32(0), cmd.SysProcAttr.Credential.Uid, "invalid Uid")
		assert.Equalf(t, gid, cmd.SysProcAttr.Credential.Gid, "invalid Gid")
	})

	t.Run("Preserve existing SysProcAttr attr", func(t *testing.T) {
		cmd := exec.Cmd{}
		cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: "/tmp"}
		xCmd := T{}
		cred, err := Credential("root", "")
		require.Nil(t, err)
		xCmd.Credential = cred
		require.Nil(t, xCmd.Update(&cmd))
		assert.Equalf(t, "/tmp", cmd.SysProcAttr.Chroot, "unexpected change")
	})

	t.Run("return error when cmd is nil", func(t *testing.T) {
		var cmd *exec.Cmd
		xCmd := T{}
		require.NotNil(t, xCmd.Update(cmd))
	})
}
