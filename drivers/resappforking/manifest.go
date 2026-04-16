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
		{
			Attr:     "StartCmd",
			Example:  "/usr/bin/touch /tmp/{fqdn}.{rindex}",
			Option:   "start",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/start"),
			Minimal:  true,
		},
		{
			Attr:     "StopCmd",
			Example:  "/usr/bin/rm -f /tmp/{fqdn}.{rindex}",
			Option:   "stop",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/stop"),
			Minimal:  true,
		},
		{
			Attr:     "CheckCmd",
			Example:  "/usr/bin/test -f /tmp/{fqdn}.{rindex}",
			Option:   "check",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/check"),
			Minimal:  true,
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
