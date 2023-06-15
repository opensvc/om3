//go:build !linux

package bootid

import "fmt"

func scan() (string, error) {
	return "", fmt.Errorf("not implemented")
}
