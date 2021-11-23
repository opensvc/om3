// +build !linux

package asset

import (
	"fmt"
	"os"
)

func New() *T {
	t := T{}
	return &t
}

func (t T) Get(s string) (interface{}, error) {
	switch s {
	case "sp_version":
		return "", ErrNotImpl
	case "enclosure":
		return "", ErrNotImpl
	case "tz":
		return TZ()
	case "mem_banks":
		return 0, ErrNotImpl
	case "mem_slots":
		return 0, ErrNotImpl
	case "fqdn":
		return os.Hostname()
	case "connect_to":
		return ConnectTo()
	default:
		return nil, fmt.Errorf("unknown asset key: %s", s)
	}
}

func Hardware() ([]Device, error) {
	all := make([]Device, 0)
	return all, nil
}
