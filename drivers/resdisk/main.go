package resdisk

import (
	"embed"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/resource"
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
		Attr:      "PromoteRW",
		Converter: "bool",
		Option:    "promote_rw",
		Text:      keywords.NewText(fs, "text/kw/promote_rw"),
	}

	BaseKeywords = append(
		manifest.SCSIPersistentReservationKeywords,
		KWPromoteRW,
	)
)
