package nmon

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

// onCertConfigUpdated checks the certificate of system/sec/cert again when
// the sec changes on this node, as when it is renewed, installed from
// elsewhere, or received on join.
func (t *Manager) onCertConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if c.Node != t.localhost {
		return
	}
	t.checkCert()
	t.refreshNodeIssues()
}

// checkCert sets the certificate issue of this node, and warns when it
// appears or changes, and says when it is gone.
//
// The listener of this node presents the certificate of system/sec/cert.
// Its peers dial it by its name, or by the address of its listener while
// verifying the certificate for its name: a certificate not naming this
// node, as one installed from elsewhere, or issued for alt_names not
// holding {clusternodes}, is refused by every peer request to this node.
func (t *Manager) checkCert() {
	issue := t.certIssue()
	switch {
	case issue == t.certIssueMsg:
		return
	case issue == "":
		t.log.Infof("%s names this node again: the peers verify its certificate", naming.SecCert)
	default:
		t.log.Warnf("%s", issue)
	}
	t.certIssueMsg = issue
}

// certIssue returns why the peers refuse the certificate of system/sec/cert
// when they request this node, and "" when they accept it, or when the sec
// has no certificate to tell, as before its bootstrap.
func (t *Manager) certIssue() string {
	if !naming.SecCert.Exists() {
		return ""
	}
	sec, err := object.NewSec(naming.SecCert, object.WithVolatile(true))
	if err != nil {
		t.log.Debugf("check %s: %s", naming.SecCert, err)
		return ""
	}
	b, err := sec.DecodeKey("certificate_chain")
	if errors.Is(err, object.ErrKeyNotExist) {
		return ""
	} else if err != nil {
		t.log.Debugf("check %s: %s", naming.SecCert, err)
		return ""
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return fmt.Sprintf("%s certificate_chain holds no certificate", naming.SecCert)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Sprintf("%s certificate_chain: %s", naming.SecCert, err)
	}
	if err := cert.VerifyHostname(t.localhost); err == nil {
		return ""
	}
	return fmt.Sprintf("%s does not name %s, so the peers refuse its certificate: it names %s, set alt_names to hold {clusternodes}, or install one naming %s",
		naming.SecCert, t.localhost, certNames(cert), t.localhost)
}

// certNames returns the names a certificate is valid for, as a list to read.
func certNames(cert *x509.Certificate) string {
	l := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		l = append(l, ip.String())
	}
	if len(l) == 0 {
		return "no name"
	}
	return strings.Join(l, ", ")
}
