package command

import (
	"syscall"

	"github.com/opensvc/om3/util/usergroup"
)

// credential returns *syscall.Credential for 'user' and 'group' string
// with associated Uid and Gid.
// when 'user' or 'group' are zero value then nil value is returned
func credential(user, group string) (*syscall.Credential, error) {
	cred := syscall.Credential{}
	var needCred bool
	if user != "" {
		userID, err := usergroup.UidFromS(user)
		if err != nil {
			return nil, err
		}
		cred.Uid = userID
		needCred = true
	}
	if group != "" {
		groupID, err := usergroup.GidFromS(group)
		if err != nil {
			return nil, err
		}
		cred.Gid = groupID
		needCred = true
	}
	if needCred {
		return &cred, nil
	} else {
		return nil, nil
	}
}
