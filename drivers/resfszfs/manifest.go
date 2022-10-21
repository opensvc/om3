package resfszfs

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/resfshost"
	"opensvc.com/opensvc/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupFS, "zfs")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword([]keywords.Keyword{
		resfshost.KeywordMountPoint,
		resfshost.KeywordDevice,
		resfshost.KeywordMountOptions,
		resfshost.KeywordStatTimeout,
		resfshost.KeywordMKFSOptions,
		resfshost.KeywordZone,
		resfshost.KeywordUser,
		resfshost.KeywordGroup,
		resfshost.KeywordPerm,
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Required:     false,
			Converter:    converters.Size,
			Scopable:     true,
			Text:         "Used by default as the refquota of the provisioned dataset. The quota, refquota, reservation and refreservation values can be expressed as a multiplier of size (example: quota=x2).",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "refquota",
			Attr:         "RefQuota",
			Required:     false,
			Scopable:     true,
			Default:      "x1",
			Text:         "The dataset 'refquota' property value to set on provision. The value can be 'none', or a size expression, or a multiplier of the size keyword value (ex: x2).",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "quota",
			Attr:         "Quota",
			Required:     false,
			Scopable:     true,
			Text:         "The dataset 'quota' property value to set on provision. The value can be 'none', or a size expression, or a multiplier of the size keyword value (ex: x2).",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "refreservation",
			Attr:         "RefReservation",
			Required:     false,
			Scopable:     true,
			Text:         "The dataset 'refreservation' property value to set on provision. The value can be 'none', or a size expression, or a multiplier of the size keyword value (ex: x2).",
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "reservation",
			Attr:         "Reservation",
			Required:     false,
			Scopable:     true,
			Text:         "The dataset 'reservation' property value to set on provision. The value can be 'none', or a size expression, or a multiplier of the size keyword value (ex: x2).",
			Provisioning: true,
		},
	}...)
	return m
}
