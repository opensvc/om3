package xexec

import (
	"opensvc.com/opensvc/util/usergroup"
	"syscall"
)

// Credential returns *syscall.Credential for 'user' and 'group' string
// with associated Uid and Gid.
// when 'user' or 'group' are zero value then nil value is returned
func Credential(user, group string) (*syscall.Credential, error) {
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
