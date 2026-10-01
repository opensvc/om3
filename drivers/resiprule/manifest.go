package resiprule

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupIP, "rule")

	kws = []*keywords.Keyword{
		{
			Attr:     "NetNS",
			Example:  "container#0",
			Option:   "netns",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
		{
			Attr:      "Spec",
			Converter: converters.Shlex,
			Example:   "from 192.168.100.0/24 table 100",
			Option:    "spec",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/spec"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.AddKeywords(kws...)
	return m
}
