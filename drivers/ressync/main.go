package ressync

import (
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
	KWMaxDelay = keywords.Keyword{
		Option:        "max_delay",
		DefaultOption: "sync_max_delay",
		Aliases:       []string{"sync_max_delay"},
		Attr:          "MaxDelay",
		Converter:     converters.Duration,
		Text:          "This sets the delay above which the status of the resource is considered down. It should be set according to your application service level agreement. The scheduler task interval should be lower than :kw:`sync_max_delay`.",
	}
	KWSchedule = keywords.Keyword{
		Option:        "schedule",
		DefaultOption: "sync_schedule",
		Attr:          "Schedule",
		Scopable:      true,
		Text:          "Set the this task run schedule. See ``/usr/share/doc/opensvc/schedule`` for the schedule syntax reference.",
		Example:       "00:00-01:00 mon",
	}

	BaseKeywords = append(
		[]keywords.Keyword{},
		KWMaxDelay,
		KWSchedule,
	)
)
