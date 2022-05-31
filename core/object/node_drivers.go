package object

import "opensvc.com/opensvc/core/driverid"

// Drivers returns the builtin drivers list
func (t Node) Drivers() (interface{}, error) {
	return driverid.List(), nil
}
