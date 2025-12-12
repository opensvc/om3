package disks

import (
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/render/tree"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// Disk is a block device at the bottom of the stacking.
	// Multipath is a special case of stacking: a multipath
	// is a Disk, but not its paths.
	Disk struct {
		ID      string   `json:"id"`
		DevPath string   `json:"devpath"`
		Size    uint64   `json:"size"`
		Vendor  string   `json:"vendor"`
		Model   string   `json:"model"`
		Type    string   `json:"type"`
		Regions []Region `json:"regions"`
	}

	// Region is a Disk part claimed by an object.
	Region struct {
		ID      string `json:"id"`
		DevPath string `json:"devpath"`
		Object  string `json:"object"`
		Size    uint64 `json:"size"`
		Group   string `json:"group"`
	}

	Disks []Disk
)

func regions(d Dev, claims ObjectsDeviceClaims) []Region {
	l := make([]Region, 0)
	unclaimedSize := d.Size
	for objPath, objClaims := range claims {
		for claimedDevName := range objClaims {
			if _relations.rootOf(claimedDevName) != d.Name {
				continue
			}
			claimedDev, ok := _devices[claimedDevName]
			if !ok {
				continue
			}
			unclaimedSize -= claimedDev.Size
			l = append(l, Region{
				ID:      claimedDev.Name,
				Size:    claimedDev.Size,
				DevPath: claimedDev.Path,
				Object:  objPath,
				Group:   "",
			})
		}
	}
	if unclaimedSize > 0 {
		l = append(l, Region{
			ID:      d.Name,
			Size:    unclaimedSize,
			DevPath: d.Path,
			Object:  "",
			Group:   "",
		})
	}
	return l
}

func (d *Disk) Used() (used uint64, err error) {
	for _, r := range d.Regions {
		used += r.Size
	}
	if used > d.Size {
		used = d.Size
	}
	return
}

// GetDisks return the list of disks visible on the node.
// Multipath paths are not considered disks.
func GetDisks(claims ObjectsDeviceClaims) (Disks, error) {
	l := make(Disks, 0)
	devices, err := GetDevices()
	if err != nil {
		return l, err
	}
	for _, d := range devices {
		if d.IsMpathPath() {
			continue
		}
		parents := d.Parents()
		vendor := d.Vendor
		model := d.Model
		if d.IsMpath() {
			if model == "" && len(parents) > 0 {
				vendor = parents[0].Vendor
				model = parents[0].Model
			}
		} else if len(parents) > 0 {
			continue
		}
		l = append(l, Disk{
			ID:      d.Name,
			DevPath: d.Path,
			Size:    d.Size,
			Vendor:  vendor,
			Model:   model,
			Type:    d.Type,
			Regions: regions(d, claims),
		})
	}
	return l, nil
}

// Render returns a human friendly string representation of the type.
func (t Disks) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText(hostname.Hostname()).SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("size")
	tree.AddColumn().AddText("vendor")
	tree.AddColumn().AddText("model")
	for _, disk := range t {
		n := tree.AddNode()
		n.AddColumn().AddText(disk.ID).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(sizeconv.BSizeCompact(float64(disk.Size)))
		n.AddColumn().AddText(disk.Vendor)
		n.AddColumn().AddText(disk.Model)
		for _, reg := range disk.Regions {
			n := n.AddNode()
			var regClaimer string
			if reg.Object == "" {
				regClaimer = "unclaimed"
			} else {
				regClaimer = reg.Object
			}
			n.AddColumn().AddText(regClaimer).SetColor(rawconfig.Color.Secondary)
			n.AddColumn().AddText(sizeconv.BSizeCompact(float64(reg.Size))).SetColor(rawconfig.Color.Secondary)
		}
	}
	return tree.Render()
}
