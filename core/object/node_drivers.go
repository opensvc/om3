package object

import "github.com/opensvc/om3/v3/core/driver"

// Drivers returns the builtin drivers list
func (t Node) Drivers() (interface{}, error) {
	return driver.List(), nil
}
