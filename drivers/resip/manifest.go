package resip

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	KeywordWaitDNS = keywords.Keyword{
		Option:    "wait_dns",
		Attr:      "WaitDNS",
		Scopable:  true,
		Default:   "0",
		Example:   "10s",
		Converter: converters.Duration,
		Text:      keywords.NewText(fs, "text/kw/wait_dns"),
	}
)
