package resdisk

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/converters"
)

type (
	T struct {
		resource.T
		resource.SCSIPersistentReservation
		PromoteRW bool
	}
)

var (
	KWPromoteRW = keywords.Keyword{
		Option:    "promote_rw",
		Attr:      "PromoteRW",
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to promote the base devices to read-write on start.",
	}
	BaseKeywords = append(manifest.SCSIPersistentReservationKeywords, KWPromoteRW)
)
