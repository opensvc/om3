package resfszfs

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resfshost"
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
func (t *T) Manifest() *manifest.T {
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
			Attr:         "Size",
			Converter:    "size",
			Option:       "size",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:         "RefQuota",
			Option:       "refquota",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/refquota"),
			Example:      "x1",
		},
		keywords.Keyword{
			Attr:         "Quota",
			Option:       "quota",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/quota"),
		},
		keywords.Keyword{
			Attr:         "RefReservation",
			Option:       "refreservation",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/refreservation"),
		},
		keywords.Keyword{
			Attr:         "Reservation",
			Option:       "reservation",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/reservation"),
		},
	)
	return m
}
