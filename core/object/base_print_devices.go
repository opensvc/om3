package object

import (
	"opensvc.com/opensvc/core/objectdevice"
	"opensvc.com/opensvc/util/device"
)

type (
	// OptsPrintDevices is the options of the PrintDevices object method.
	OptsPrintDevices struct {
		Global OptsGlobal
	}
	devExposer interface {
		ExposedDevices() []*device.T
	}
)

// PrintDevices display the object scheduling table
func (t *Base) PrintDevices(options OptsPrintDevices) objectdevice.L {
	l := objectdevice.NewList()
	for _, r := range t.Resources() {
		var i interface{} = r
		o, ok := i.(devExposer)
		if !ok {
			continue
		}
		for _, dev := range o.ExposedDevices() {
			manifest := r.Manifest()
			l = l.Add(objectdevice.T{
				Device:      dev,
				Role:        objectdevice.RoleExposed,
				RID:         r.RID(),
				DriverGroup: manifest.Group,
				DriverName:  manifest.Name,
				ObjectPath:  t.Path,
			})
		}
	}
	return l
}
