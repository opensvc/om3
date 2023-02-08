package resappforking

import (
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	Keywords = []keywords.Keyword{
		{
			Option:    "start_timeout",
			Attr:      "StartTimeout",
			Converter: converters.Duration,
			Scopable:  true,
			Text: "Wait for <duration> before declaring the app launcher start action a failure." +
				"  Takes precedence over :kw:`timeout`. If neither :kw:`timeout` nor :kw:`start_timeout` is set," +
				" the agent waits indefinitely for the app launcher to return." +
				" A timeout can be coupled with :kw:`optional=true to not abort a service instance start when an app" +
				" launcher did not return.",
			Example: "180",
		},
	}
)
