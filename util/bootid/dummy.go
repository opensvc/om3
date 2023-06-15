//go:build !linux

package bootid

func scan() (string, error) {
	return "", fmt.Errorf("not implemented")
}
