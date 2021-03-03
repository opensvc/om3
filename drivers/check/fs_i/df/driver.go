package main

import "opensvc.com/opensvc/core/check"

// Check return a list of filesystem usage per active mount point
func (t Type) Check() ([]*check.Result, error) {
	r, err := t.parseDF()
	if err != nil {
		return nil, err
	}
	for _, e := range r {
		e.DriverGroup = DriverGroup
		e.DriverName = DriverName
	}
	return r, nil
}

// ObjectPath returns the path of the first object using the mountpoint
// passed as argument
func (t Type) ObjectPath(mnt string) string {
	return ""
}
