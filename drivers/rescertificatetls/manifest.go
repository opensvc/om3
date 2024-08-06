package rescertificatetls

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupCertificate, "tls")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		keywords.Keyword{
			Option:   "certificate_secret",
			Attr:     "CertificateSecret",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/certificate_secret"),
		},
		keywords.Keyword{
			Option:   "validation_secret",
			Attr:     "ValidationSecret",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/validation_secret"),
		},
		keywords.Keyword{
			Option:   "certificate_chain_filename",
			Attr:     "CertificateChainFilename",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/certificate_chain_filename"),
		},
		keywords.Keyword{
			Option:   "private_key_filename",
			Attr:     "PrivateKeyFilename",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/private_key_filename"),
		},
		keywords.Keyword{
			Option:   "certificate_chain_inline_string",
			Attr:     "CertificateChainInlineString",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/certificate_chain_inline_string"),
		},
		keywords.Keyword{
			Option:   "private_key_inline_string",
			Attr:     "PrivateKeyInlineString",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/private_key_inline_string"),
		},
	)
	return m
}
