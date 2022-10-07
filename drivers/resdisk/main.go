package resdisk

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/converters"
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
	BaseKeywords = append(resource.SCSIPersistentReservationKeywords, KWPromoteRW)
)
