package check

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/device"
)

type (
	// Checker exposes what can be done with a check
	Checker interface {
		Check(ctx context.Context, objs []interface{}) (*ResultSet, error)
	}

	// T is the check type
	T struct {
		Name string
	}

	// Result is the structure eventually collected for aggregation.
	Result struct {
		DriverGroup string `json:"type"`
		DriverName  string `json:"driver"`
		Path        string `json:"path"`
		Instance    string `json:"instance"`
		Unit        string `json:"unit"`
		Value       int64  `json:"value"`
	}

	header interface {
		Head() string
	}
	resourceLister interface {
		Resources() resource.Drivers
	}
	subDeviceser interface {
		SubDevices(context.Context) device.L
	}
	exposedDeviceser interface {
		ExposedDevices(context.Context) device.L
	}
)

var checkers = make([]Checker, 0)

// UnRegisterAll unregister all registered checkers
func UnRegisterAll() {
	checkers = make([]Checker, 0)
}

func Register(c Checker) {
	checkers = append(checkers, c)
}

func (r T) String() string {
	return fmt.Sprintf("<Check %s>", r.Name)
}

// Check returns a result list
func Check(ctx context.Context, r Checker, objs []interface{}) error {
	data, err := r.Check(ctx, objs)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(data)
	return nil
}

// ObjectPathClaimingDir returns the first object using the directory: the
// object whose head it is, or else the object of a provisioned resource whose
// head it is.
func ObjectPathClaimingDir(ctx context.Context, p string, objs []interface{}) string {
	return headIndexOf(ctx, objs).path(p)
}

// headIndex is the object each head belongs to: the heads of the objects,
// then the heads of their provisioned resources, the first object found for
// a head winning.
type headIndex struct {
	objects   map[string]string
	resources map[string]string
}

func (t headIndex) path(p string) string {
	if s, ok := t.objects[p]; ok {
		return s
	}
	return t.resources[p]
}

func newHeadIndex(ctx context.Context, objs []interface{}) headIndex {
	t := headIndex{objects: make(map[string]string), resources: make(map[string]string)}
	for _, obj := range objs {
		h, ok := obj.(header)
		if !ok {
			continue
		}
		if p := h.Head(); p != "" {
			if _, ok := t.objects[p]; !ok {
				t.objects[p] = fmt.Sprint(obj)
			}
		}
	}
	for _, obj := range objs {
		b, ok := obj.(resourceLister)
		if !ok {
			continue
		}
		for _, r := range b.Resources() {
			h, ok := r.(header)
			if !ok {
				continue
			}
			if v, err := r.Provisioned(ctx); err != nil || v == provisioned.False {
				continue
			}
			if p := h.Head(); p != "" {
				if _, ok := t.resources[p]; !ok {
					t.resources[p] = fmt.Sprint(obj)
				}
			}
		}
	}
	return t
}

// indexes holds the head and device indexes of a check run, built once for
// the objects of the run and shared by its checkers: asking each object and
// resource for its head or devices runs their tools, a few seconds for a
// hundred objects, which every check result paid when looked up one by one.
type indexes struct {
	headOnce   sync.Once
	head       headIndex
	deviceOnce sync.Once
	device     DeviceIndex
}

var (
	indexesMu sync.Mutex

	// indexesByObjects are the indexes of the runs in progress, by the
	// first element of the objects of the run, the slice every checker of
	// a run is passed.
	indexesByObjects = make(map[*interface{}]*indexes)
)

func indexesOf(objs []interface{}) *indexes {
	if len(objs) == 0 {
		return &indexes{}
	}
	indexesMu.Lock()
	defer indexesMu.Unlock()
	k := &objs[0]
	i, ok := indexesByObjects[k]
	if !ok {
		i = &indexes{}
		indexesByObjects[k] = i
	}
	return i
}

// forgetIndexes drops the indexes of the run of the objects, once its
// checkers ended.
func forgetIndexes(objs []interface{}) {
	if len(objs) == 0 {
		return
	}
	indexesMu.Lock()
	defer indexesMu.Unlock()
	delete(indexesByObjects, &objs[0])
}

func headIndexOf(ctx context.Context, objs []interface{}) headIndex {
	i := indexesOf(objs)
	i.headOnce.Do(func() { i.head = newHeadIndex(ctx, objs) })
	return i.head
}

// DeviceIndexOf returns the device index of the objects, built once for the
// checkers of a run.
func DeviceIndexOf(ctx context.Context, objs []interface{}) DeviceIndex {
	i := indexesOf(objs)
	i.deviceOnce.Do(func() { i.device = NewDeviceIndex(ctx, objs) })
	return i.device
}

// DeviceIndex is the object each device used or exposed by a disk or a fs
// resource belongs to, by device path, its links resolved, as
// /dev/mapper/<name> and the /dev/dm-<n> it leads to.
//
// A checker reporting devices builds it once per check: asking every
// resource for its devices for each device reported would run their tools
// as many times.
type DeviceIndex map[string]string

// NewDeviceIndex returns the index of the devices of the disk and fs
// resources of the objects. The devices of the other resources, as a
// container, are not the ones a node device is attributed by.
func NewDeviceIndex(ctx context.Context, objs []interface{}) DeviceIndex {
	m := make(DeviceIndex)
	for _, obj := range objs {
		b, ok := obj.(resourceLister)
		if !ok {
			continue
		}
		for _, r := range b.Resources() {
			switch r.DriverID().Group {
			case driver.GroupDisk, driver.GroupFS:
			default:
				continue
			}
			var l device.L
			if i, ok := r.(subDeviceser); ok {
				l = append(l, i.SubDevices(ctx)...)
			}
			if i, ok := r.(exposedDeviceser); ok {
				l = append(l, i.ExposedDevices(ctx)...)
			}
			for _, dev := range l {
				p := realPath(dev.Path())
				if _, ok := m[p]; !ok {
					m[p] = fmt.Sprint(obj)
				}
			}
		}
	}
	return m
}

// Path returns the object of the first of the devices it knows, and empty
// when it knows none.
func (t DeviceIndex) Path(devs ...string) string {
	for _, dev := range devs {
		if dev == "" {
			continue
		}
		if p, ok := t[realPath(dev)]; ok {
			return p
		}
	}
	return ""
}

// ObjectPathWithResource returns the first object having a resource match
// selects, and empty when none has one.
func ObjectPathWithResource(ctx context.Context, objs []interface{}, match func(resource.Driver) bool) string {
	for _, obj := range objs {
		b, ok := obj.(resourceLister)
		if !ok {
			continue
		}
		for _, r := range b.Resources() {
			if match(r) {
				return fmt.Sprint(obj)
			}
		}
	}
	return ""
}

func realPath(p string) string {
	if s, err := filepath.EvalSymlinks(p); err == nil {
		return s
	}
	return p
}
