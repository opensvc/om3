package resfszfs

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resfshost"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupFS, "zfs")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
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
			Text:         keywords.NewText(fs, "text/kw/size"),
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "refquota",
			Attr:         "RefQuota",
			Required:     false,
			Scopable:     true,
			Default:      "x1",
			Text:         keywords.NewText(fs, "text/kw/refquota"),
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "quota",
			Attr:         "Quota",
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/quota"),
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "refreservation",
			Attr:         "RefReservation",
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/refreservation"),
			Provisioning: true,
		},
		keywords.Keyword{
			Option:       "reservation",
			Attr:         "Reservation",
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/reservation"),
			Provisioning: true,
		},
	)
	return m
}
