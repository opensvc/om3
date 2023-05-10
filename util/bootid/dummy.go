//go:build !linux

package bootid

import "github.com/pkg/errors"

func scan() (string, error) {
	return "", errors.New("not implemented")
}
