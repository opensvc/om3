// Package credential handles the <username>:<password> pair a node is handed
// to create a user with, once it is alone in its own cluster and no longer
// reachable with the credentials of the cluster it left.
package credential

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalid is returned for a credential that is not a
	// <username>:<password> pair with both halves set.
	ErrInvalid = errors.New("invalid credential")
)

// Parse splits a <username>:<password> credential.
//
// The separator is the first colon, so a password is free to contain one. A
// username is not: it is the name of the usr object to create, and a colon is
// not valid there.
func Parse(s string) (username, password string, err error) {
	username, password, ok := strings.Cut(s, ":")
	if !ok {
		return "", "", fmt.Errorf("%w: expected <username>:<password>", ErrInvalid)
	}
	if username == "" {
		return "", "", fmt.Errorf("%w: username is empty", ErrInvalid)
	}
	if password == "" {
		return "", "", fmt.Errorf("%w: password is empty", ErrInvalid)
	}
	return username, password, nil
}
