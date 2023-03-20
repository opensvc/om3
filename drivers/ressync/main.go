package ressync

import (
	"embed"
	"time"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/util/converters"
)

type (
	T struct {
		resource.T
		MaxDelay *time.Duration
		Schedule string
	}
)

var (
	//go:embed text
	fs embed.FS

	KWMaxDelay = keywords.Keyword{
		Option:        "max_delay",
		DefaultOption: "sync_max_delay",
		Aliases:       []string{"sync_max_delay"},
		Attr:          "MaxDelay",
		Converter:     converters.Duration,
		Text:          keywords.NewText(fs, "text/kw/max_delay"),
	}
	KWSchedule = keywords.Keyword{
		Option:        "schedule",
		DefaultOption: "sync_schedule",
		Attr:          "Schedule",
		Scopable:      true,
		Example:       "00:00-01:00 mon",
		Text:          keywords.NewText(fs, "text/kw/schedule"),
	}

	BaseKeywords = append(
		[]keywords.Keyword{},
		KWMaxDelay,
		KWSchedule,
	)
)
