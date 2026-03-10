package resfszfs

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resfshost"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupFS, "zfs")

	kws = []*keywords.Keyword{
		&resfshost.KeywordMountPoint,
		&resfshost.KeywordDevice,
		&resfshost.KeywordMountOptions,
		&resfshost.KeywordStatTimeout,
		&resfshost.KeywordMKFSOptions,
		&resfshost.KeywordZone,
		&resfshost.KeywordUser,
		&resfshost.KeywordGroup,
		&resfshost.KeywordPerm,
		{
			Attr:         "Size",
			Converter:    "size",
			Option:       "size",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		{
			Attr:         "RefQuota",
			Option:       "refquota",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/refquota"),
			Example:      "x1",
		},
		{
			Attr:         "Quota",
			Option:       "quota",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/quota"),
		},
		{
			Attr:         "RefReservation",
			Option:       "refreservation",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/refreservation"),
		},
		{
			Attr:         "Reservation",
			Option:       "reservation",
			Provisioning: true,
			Required:     false,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/reservation"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(kws...)
	return m
}
