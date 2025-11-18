//go:build !linux

package resip

import "fmt"

func AllocateDevLabel(dev string) (string, error) {
	return "", fmt.Errorf("not implemented on this platform")
}

func SplitDevLabel(s string) (string, string) {
	return s, ""
}
