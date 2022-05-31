package object

import "opensvc.com/opensvc/core/driver"

// Drivers returns the builtin drivers list
func (t Node) Drivers() (interface{}, error) {
	return driver.List(), nil
}
