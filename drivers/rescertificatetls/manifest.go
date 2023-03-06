package rescertificatetls

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
)

var (
	drvID = driver.NewID(driver.GroupCertificate, "tls")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		keywords.Keyword{
			Option:   "certificate_secret",
			Attr:     "CertificateSecret",
			Scopable: true,
			Text:     "The name of the secret object name hosting the certificate files. The secret must have the certificate_chain and server_key keys set. This setting makes the certificate served to envoy via the secret discovery service, which allows its live rotation.",
		},
		keywords.Keyword{
			Option:   "validation_secret",
			Attr:     "ValidationSecret",
			Scopable: true,
			Text:     "The name of the secret object name hosting the certificate autority files for certificate_secret validation. The secret must have the trusted_ca and verify_certificate_hash keys set. This setting makes the validation data served to envoy via the secret discovery service, which allows certificates live rotation.",
		},
		keywords.Keyword{
			Option:   "certificate_chain_filename",
			Attr:     "CertificateChainFilename",
			Scopable: true,
			Text:     "Local filesystem data source of the TLS certificate chain.",
		},
		keywords.Keyword{
			Option:   "private_key_filename",
			Attr:     "PrivateKeyFilename",
			Scopable: true,
			Text:     "Local filesystem data source of the TLS private key.",
		},
		keywords.Keyword{
			Option:   "certificate_chain_inline_string",
			Attr:     "CertificateChainInlineString",
			Scopable: true,
			Text:     "String inlined data source of the TLS certificate chain.",
		},
		keywords.Keyword{
			Option:   "private_key_inline_string",
			Attr:     "PrivateKeyInlineString",
			Scopable: true,
			Text:     "String inlined filesystem data source of the TLS private key. A reference to a secret for example.",
		},
	)
	return m
}
