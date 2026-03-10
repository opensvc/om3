package resappforking

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resapp"
)

var (
	drvID = driver.NewID(driver.GroupApp, "forking")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Attr:      "StartTimeout",
			Converter: "duration",
			Example:   "180",
			Option:    "start_timeout",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/start_timeout"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
	)
	m.AddKeywords(resapp.BaseKeywords...)
	m.AddKeywords(resapp.UnixKeywords...)
	m.AddKeywords(kws...)
	return m
}
