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
		Attr:      "WaitDNS",
		Converter: converters.Duration,
		Default:   "0",
		Example:   "10s",
		Option:    "wait_dns",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/wait_dns"),
	}
)
