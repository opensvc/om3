package rescontainerkvm

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// GroupInstall holds how the provision makes the guest: the virt-install
// command the keywords describing the guest make, given a cloud image to
// write on its first disk or an installer to boot, or the command virtinst
// says all of.
//
// Its strategies are the ones the provision refuses to mix, by specError:
// virtinst excludes every keyword describing the guest, the ones of the
// machine, storage and network groups included, and cdrom excludes image and
// cloud_init. A guest of neither image nor cdrom boots its first disk as it
// is, with the cloud_init of the image strategy or none, which no strategy
// of its own says.
var GroupInstall = keywords.Group{
	ID:    "install",
	Name:  "Installation",
	Text:  keywords.NewText(fs, "text/group/install"),
	Order: 30,
	Strategies: []keywords.Strategy{
		{
			ID:       "image",
			Name:     "From a cloud image",
			Text:     keywords.NewText(fs, "text/strategy/install/image"),
			Uses:     []string{"image", "cloud_init"},
			Excludes: []string{"cdrom", "virtinst"},
		},
		{
			ID:       "cdrom",
			Name:     "From an installer",
			Text:     keywords.NewText(fs, "text/strategy/install/cdrom"),
			Uses:     []string{"cdrom"},
			Excludes: []string{"image", "cloud_init", "virtinst"},
		},
		{
			ID:       "virtinst",
			Name:     "From a command",
			Text:     keywords.NewText(fs, "text/strategy/install/virtinst"),
			Uses:     []string{"virtinst"},
			Excludes: []string{"vcpus", "memory", "disk", "bridge", "network", "osinfo", "image", "cdrom", "cloud_init", "virtinst_args"},
		},
	},
}
