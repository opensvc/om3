package disks

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/device"
)

type (
	// DeviceClaims is a dedup-map indexed by block device path.
	DeviceClaims map[string]interface{}

	// ObjectsDeviceClaims is a map of DeviceClaims indexed by object path.
	// It links objects to block device paths.
	ObjectsDeviceClaims map[string]DeviceClaims

	// deviceClaimer is an optional interface a Resource driver can implement
	// to advertise to the core which block device it claims. Used by pushdisks
	// to attribute disks regions to objects.
	deviceClaimer interface {
		ClaimedDevices() device.L
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
)

func NewObjectsDeviceClaims() ObjectsDeviceClaims {
	return make(ObjectsDeviceClaims)
}

func (t DeviceClaims) AddPath(s string) {
	dev, err := getDeviceFromPath(s)
	if err != nil {
		return
	}
	for current := range t {
		if _relations.leafOf(dev.Name, current) {
			// A parent device idev.Name already claimed
			return
		}
		if _relations.leafOf(current, dev.Name) {
			// Drop child device claims this new claim will encompass
			delete(t, current)
		}
	}
	t[dev.Name] = nil
}

func (t DeviceClaims) AddResource(r interface{}) {
	i, ok := r.(deviceClaimer)
	if !ok {
		return
	}
	for _, d := range i.ClaimedDevices() {
		t.AddPath(d.Path())
	}
}

func (t ObjectsDeviceClaims) AddObjects(objs ...interface{}) {
	for _, obj := range objs {
		path := fmt.Sprint(obj)
		lister, ok := obj.(resourceLister)
		if !ok {
			continue
		}
		if _, ok := t[path]; !ok {
			t[path] = make(DeviceClaims)
		}
		for _, r := range lister.Resources() {
			t[path].AddResource(r)
		}
	}
}
