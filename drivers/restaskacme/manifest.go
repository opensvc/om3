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
			Example:   "web ./sec/api ns2/sec/shared ./sec/web-*",
			Option:    "secs",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/secs"),
			Validate:  validateSecs,
		},
		{
			Attr:     "Directory",
			Example:  "letsencrypt",
			Option:   "directory",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/directory"),
		},
		{
			Attr:     "Email",
			Example:  "admin@example.com",
			Option:   "email",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/email"),
		},
		{
			Attr:     "DomainsFrom",
			Example:  "volume#1:/haproxy/conf.d/*.cfg",
			Option:   "domains_from",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/domains_from"),
			Validate: validateDomainsFrom,
		},
		{
			Attr:     "DomainsTo",
			Example:  "./sec/{name}-*",
			Option:   "domains_to",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/domains_to"),
			Validate: validateDomainsTo,
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
	driver.SetDoc(drvID, "Renews the ACME certificates of the object, as a scheduled task.")
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
