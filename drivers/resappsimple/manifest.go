package resappsimple

import (
	"embed"
	_ "embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resapp"
)

var (
	drvID = driver.NewID(driver.GroupApp, "simple")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Attr:     "StartCmd",
			Option:   "start",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/start"),
			Minimal:  true,
			Example:  "/usr/bin/sleep 600",
		},
		{
			Attr:     "StopCmd",
			Example:  "/usr/local/bin/stop",
			Option:   "stop",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/stop"),
		},
		{
			Attr:     "CheckCmd",
			Example:  "/usr/local/bin/check",
			Option:   "check",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/check"),
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
