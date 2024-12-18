package rescertificatetls

import (
	"context"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
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

func New() resource.Driver {
	t := &T{}
	return t
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

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t T) Label(_ context.Context) string {
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

// StatusInfo implements resource.StatusInfoer
func (t T) StatusInfo(_ context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	data["certificate_secret"] = t.CertificateSecret
	data["validation_secret"] = t.ValidationSecret
	data["certificate_chain_filename"] = t.CertificateChainFilename
	data["private_key_filename"] = t.PrivateKeyFilename
	data["certificate_chain_inline_string"] = t.CertificateChainInlineString
	data["private_key_inline_string"] = t.PrivateKeyInlineString
	return data
}
