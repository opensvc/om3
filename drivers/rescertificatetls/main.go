package resfsdir

import (
	"context"

	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
)

const (
	driverGroup = driver.GroupCertificate
	driverName  = "tls"
)

type (
	T struct {
		resource.T
		CertificateSecret            string `json:"certificate_secret,omitempty"`
		ValidationSecret             string `json:"validation_secret,omitempty"`
		CertificateChainFilename     string `json:"certificate_chain_filename,omitempty"`
		PrivateKeyFilename           string `json:"private_key_filename,omitempty"`
		CertificateChainInlineString string `json:"certificate_chain_inline_string,omitempty"`
		PrivateKeyInlineString       string `json:"private_key_inline_string,omitempty"`
	}
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "certificate_secret",
			Attr:     "CertificateSecret",
			Scopable: true,
			Text:     "The name of the secret object name hosting the certificate files. The secret must have the certificate_chain and server_key keys set. This setting makes the certificate served to envoy via the secret discovery service, which allows its live rotation.",
		},
		{
			Option:   "validation_secret",
			Attr:     "ValidationSecret",
			Scopable: true,
			Text:     "The name of the secret object name hosting the certificate autority files for certificate_secret validation. The secret must have the trusted_ca and verify_certificate_hash keys set. This setting makes the validation data served to envoy via the secret discovery service, which allows certificates live rotation.",
		},
		{
			Option:   "certificate_chain_filename",
			Attr:     "CertificateChainFilename",
			Scopable: true,
			Text:     "Local filesystem data source of the TLS certificate chain.",
		},
		{
			Option:   "private_key_filename",
			Attr:     "PrivateKeyFilename",
			Scopable: true,
			Text:     "Local filesystem data source of the TLS private key.",
		},
		{
			Option:   "certificate_chain_inline_string",
			Attr:     "CertificateChainInlineString",
			Scopable: true,
			Text:     "String inlined data source of the TLS certificate chain.",
		},
		{
			Option:   "private_key_inline_string",
			Attr:     "PrivateKeyInlineString",
			Scopable: true,
			Text:     "String inlined filesystem data source of the TLS private key. A reference to a secret for example.",
		},
	}...)
	return m
}

func (t T) Start(ctx context.Context) error {
	return nil
}

func (t T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

func (t T) Label() string {
	if t.CertificateSecret != "" {
		return "from sec " + t.CertificateSecret
	}
	if t.CertificateChainFilename != "" {
		return "from host file"
	}
	if t.CertificateChainInlineString != "" {
		return "from conf"
	}
	return "empty"
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) StatusInfo() map[string]interface{} {
	data := make(map[string]interface{})
	data["certificate_secret"] = t.CertificateSecret
	data["validation_secret"] = t.ValidationSecret
	data["certificate_chain_filename"] = t.CertificateChainFilename
	data["private_key_filename"] = t.PrivateKeyFilename
	data["certificate_chain_inline_string"] = t.CertificateChainInlineString
	data["private_key_inline_string"] = t.PrivateKeyInlineString
	return data
}
