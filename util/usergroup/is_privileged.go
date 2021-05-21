package usergroup

import "os/user"

// IsPrivileged return true if current user is privileged user
func IsPrivileged() (bool, error) {
	if currentUser, err := user.Current(); err != nil {
		return false, err
	} else if currentUser.Username == "root" {
		return true, nil
	} else {
		return false, nil
	}
}
