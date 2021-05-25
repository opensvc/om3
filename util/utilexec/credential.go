// Package utilexec provide utilities around os/exec package
//

package utilexec

import (
	"opensvc.com/opensvc/util/usergroup"
	"os/exec"
	"syscall"
)

// SetCredential update cmd.SysProcAttr for 'user' and 'group' string
// with associated Uid and Gid.
// when 'user' or 'group' are zero value then associated credential is unset
// When both 'user' and 'gid' are empty string cmd.SysProcAttr credential is not
// updated.
func SetCredential(cmd *exec.Cmd, user, group string) error {
	if credential, err := getCredential(user, group); err != nil {
		return err
	} else if credential != nil {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Credential = credential
	}
	return nil
}

func getCredential(user, group string) (*syscall.Credential, error) {
	cred := syscall.Credential{}
	var needCred bool
	if user != "" {
		userId, err := usergroup.UidFromS(user)
		if err != nil {
			return nil, err
		}
		cred.Uid = userId
		needCred = true
	}
	if group != "" {
		groupId, err := usergroup.GidFromS(group)
		if err != nil {
			return nil, err
		}
		cred.Gid = groupId
		needCred = true
	}
	if needCred {
		return &cred, nil
	} else {
		return nil, nil
	}
}
