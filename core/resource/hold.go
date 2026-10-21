package resource

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// Hold is a value a resource holds on a node once provisioned and
	// started, which no other object's resource may hold on that node: a
	// mount point, a block device, the name of a volume group, an address.
	//
	// The class tells values apart across drivers: a block device an fs
	// mounts and a block device a volume group is made of are the same
	// thing, whichever driver says it.
	Hold struct {
		Class string `json:"class"`
		Value string `json:"value"`
	}

	// Holds are the values a resource holds, published as their values by
	// class, the class said by its code:
	//
	//	{"bdev": ["/dev/sdb", "/dev/sdc"], "mnt": ["/srv/app"]}
	//
	// A resource holds a few values of fewer classes, each said once.
	Holds []Hold

	// HoldSource reads the configuration of a resource for the values it
	// holds, as evaluated for one node.
	HoldSource interface {
		// Get returns the value of an option of the resource section,
		// and false when it does not evaluate without running anything:
		// a reference to what another resource exposes once
		// provisioned, as {disk#1.exposed_devs}, or to the head of a
		// volume. Such a value is held by the resource it comes from,
		// whose own holds say it.
		Get(option string) (string, bool)

		// Path is the path of the object of the resource.
		Path() naming.Path

		// RID is the identifier of the resource.
		RID() string
	}

	// Holder is implemented by the drivers whose resources hold values.
	// It is called on a driver allocated and not configured: the values
	// come from the source alone, and nothing runs.
	Holder interface {
		Holds(HoldSource) []Hold
	}
)

// The classes of the values resources hold.
const (
	HoldMountPoint    = "mount point"
	HoldBlockDevice   = "block device"
	HoldVG            = "volume group"
	HoldLV            = "logical volume"
	HoldZpool         = "zpool"
	HoldZfsDataset    = "zfs dataset"
	HoldMD            = "md"
	HoldFile          = "file"
	HoldDMName        = "device mapper name"
	HoldDRBD          = "drbd resource"
	HoldRadosImage    = "rados image"
	HoldDiskID        = "disk id"
	HoldContainerName = "container name"
	HoldIPAddress     = "ip address"
	HoldExport        = "export"
	HoldVolume        = "volume"
)

// holdClassCodes are the codes the classes are published as.
var holdClassCodes = map[string]string{
	HoldMountPoint:    "mnt",
	HoldBlockDevice:   "bdev",
	HoldVG:            "vg",
	HoldLV:            "lv",
	HoldZpool:         "zpool",
	HoldZfsDataset:    "zfs",
	HoldMD:            "md",
	HoldFile:          "file",
	HoldDMName:        "dm",
	HoldDRBD:          "drbd",
	HoldRadosImage:    "rbd",
	HoldDiskID:        "diskid",
	HoldContainerName: "ctr",
	HoldIPAddress:     "ip",
	HoldExport:        "export",
	HoldVolume:        "vol",
}

var holdClassesByCode = func() map[string]string {
	m := make(map[string]string, len(holdClassCodes))
	for class, code := range holdClassCodes {
		m[code] = class
	}
	return m
}()

// MarshalJSON publishes the holds as their values by class code. A class
// without a code, published by a newer node, is said as it is.
func (t Holds) MarshalJSON() ([]byte, error) {
	m := make(map[string][]string)
	for _, h := range t {
		code, ok := holdClassCodes[h.Class]
		if !ok {
			code = h.Class
		}
		m[code] = append(m[code], h.Value)
	}
	return json.Marshal(m)
}

// UnmarshalJSON reads the holds MarshalJSON publishes, sorted by class and
// value.
func (t *Holds) UnmarshalJSON(b []byte) error {
	var m map[string][]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	l := make(Holds, 0)
	for _, code := range slices.Sorted(maps.Keys(m)) {
		class, ok := holdClassesByCode[code]
		if !ok {
			class = code
		}
		for _, v := range m[code] {
			l = append(l, Hold{Class: class, Value: v})
		}
	}
	slices.SortStableFunc(l, func(a, b Hold) int {
		if c := strings.Compare(a.Class, b.Class); c != 0 {
			return c
		}
		return strings.Compare(a.Value, b.Value)
	})
	*t = l
	return nil
}

// ObjectHold is a value a resource of an object holds, and where: on the
// nodes listed, or on every cluster node, for a shared resource, whose
// values are reachable from every node. A node joining the cluster is held
// on by a cluster-wide hold with nothing republished.
type ObjectHold struct {
	RID string `json:"rid"`
	Hold
	Nodes   []string `json:"nodes,omitempty"`
	Cluster bool     `json:"cluster,omitempty"`
}

// HeldOn says whether the hold is held on the node.
func (t ObjectHold) HeldOn(node string) bool {
	if t.Cluster {
		return true
	}
	for _, n := range t.Nodes {
		if n == node {
			return true
		}
	}
	return false
}

// Overlaps says whether the two holds hold the same value on a node.
func (t ObjectHold) Overlaps(o ObjectHold) bool {
	if t.Hold != o.Hold {
		return false
	}
	if t.Cluster || o.Cluster {
		return true
	}
	for _, n := range t.Nodes {
		if o.HeldOn(n) {
			return true
		}
	}
	return false
}

// DevicePaths returns the block device paths of s, a list of words: the ones
// naming a device under /dev, not a pattern of them, cleaned.
func DevicePaths(s string) []string {
	l := make([]string, 0)
	for _, w := range strings.Fields(s) {
		if !strings.HasPrefix(w, "/dev/") || strings.ContainsAny(w, "*?[{") {
			continue
		}
		l = append(l, filepath.Clean(w))
	}
	return l
}

// HoldsOf returns a hold of the class for each value.
func HoldsOf(class string, values ...string) []Hold {
	l := make([]Hold, 0, len(values))
	for _, v := range values {
		if v != "" {
			l = append(l, Hold{Class: class, Value: v})
		}
	}
	return l
}
