package object

import (
	"opensvc.com/opensvc/core/resource"
)

// Drivers returns the builtin drivers list
func (t Node) Drivers() (interface{}, error) {
	return resource.DriverIDList(), nil
}
