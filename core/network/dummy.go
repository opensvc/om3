//go:build !linux

package network

func setupFW(_ logger, _ []Networker) error {
	return nil
}
