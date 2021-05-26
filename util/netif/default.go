// +build !linux

package netif

import "errors"

func HasCarrier(name string) (bool, error) {
	return false, errors.New("netif.HasCarrier() not implemented")
}
