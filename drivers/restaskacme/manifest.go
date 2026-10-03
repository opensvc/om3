package restaskacme

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/restask"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupTask, "acme")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Attr:      "Secs",
			Converter: converters.List,
			Example:   "web ./sec/api ns2/sec/shared",
			Option:    "secs",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/secs"),
		},
		{
			Attr:     "Webroot",
			Example:  "volume#1:/haproxy/acme-challenges",
			Option:   "webroot",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/webroot"),
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
	)
	m.AddKeywords(restask.Keywords...)
	m.AddKeywords(kws...)
	return m
}
