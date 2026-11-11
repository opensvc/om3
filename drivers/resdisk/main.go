package resdisk

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/converters"
)

type (
	T struct {
		resource.T
		resource.Restart
		resource.SCSIPersistentReservation
		PromoteRW bool
	}
)

var (
	//go:embed text
	fs embed.FS

	KWPromoteRW = keywords.Keyword{
		Attr:       "PromoteRW",
		Converter:  converters.Bool,
		Deprecated: "v3.0.0-rc62",
		Option:     "promote_rw",
		Group:      GroupAccess.ID,
		Text:       keywords.NewText(fs, "text/kw/promote_rw"),
	}

	BaseKeywords = append(
		manifest.SCSIPersistentReservationKeywords,
		&KWPromoteRW,
	)
)
