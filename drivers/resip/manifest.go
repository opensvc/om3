package resip

import (
	"embed"

	"github.com/opensvc/om3/core/keywords"
)

var (
	//go:embed text
	fs embed.FS

	KeywordWaitDNS = keywords.Keyword{
		Attr:      "WaitDNS",
		Converter: "duration",
		Default:   "0",
		Example:   "10s",
		Option:    "wait_dns",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/wait_dns"),
	}
)
