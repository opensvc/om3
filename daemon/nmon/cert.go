package nmon

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

const (
	// certCheckInterval is the interval the expiry of the certificates is
	// checked at, besides the changes of their secs.
	certCheckInterval = time.Hour

	// certExpiryWarning is how long before its expiry the certificate of
	// system/sec/cert is an issue: the speaker renews it 30 days before,
	// so it is one when that failed, or when it was installed from
	// elsewhere.
	certExpiryWarning = 14 * 24 * time.Hour

	// caExpiryWarning is how long before its expiry the certificate of
	// system/sec/ca is an issue: the speaker renews it a fifth of its
	// validity before, a year at most.
	caExpiryWarning = 90 * 24 * time.Hour
)

// onCertChanged checks the certificates of system/sec/cert and system/sec/ca
// again when one of the secs changes on this node, as when it is renewed,
// installed from elsewhere, or received on join.
//
// A deletion is not a change of what the peers are presented: the listener
// keeps the files installed, until the daemon starts again and bootstraps the
// sec, which the check of the config loaded then sees. The issues are kept as
// they are meanwhile.
func (t *Manager) onCertChanged(node string) {
	if node != t.localhost {
		return
	}
	t.checkCert()
	t.refreshNodeIssues()
}

// checkCert sets the certificate issues of this node, warns when one
// appears or changes, and says when one is gone.
//
// The listener of this node presents the certificate of system/sec/cert.
// Its peers dial it by its name, or by the address of its listener while
// verifying the certificate for its name: a certificate not naming this
// node, as one installed from elsewhere, or issued for alt_names not
// holding {clusternodes}, is refused by every peer request to this node.
// An expired certificate is refused too, as every certificate system/sec/ca
// signed is once it expires.
func (t *Manager) checkCert() {
	issues := t.certIssuesNow(time.Now())
	for _, issue := range t.certIssues {
		if !slices.Contains(issues, issue) {
			t.log.Infof("the certificate issue is cleared: %s", issue)
		}
	}
	for _, issue := range issues {
		if !slices.Contains(t.certIssues, issue) {
			t.log.Warnf("%s", issue)
		}
	}
	t.certIssues = issues
}

// certIssuesNow returns the certificate issues of this node at now.
func (t *Manager) certIssuesNow(now time.Time) []string {
	var issues []string
	if issue := t.certIssue(now); issue != "" {
		issues = append(issues, issue)
	}
	if issue := caIssue(now); issue != "" {
		issues = append(issues, issue)
	}
	issues = append(issues, trustedCAIssues(now)...)
	return issues
}

// certIssue returns why the peers refuse, or are soon to refuse, the
// certificate of system/sec/cert when they request this node, and "" when
// they accept it, or when the sec has no certificate to tell, as before its
// bootstrap.
//
// The certificate is checked with its private key, as the listener loads
// them: a pair that does not match is not loaded, and the listener keeps
// presenting the certificate it had, which this one does not tell.
func (t *Manager) certIssue(now time.Time) string {
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
	if key, err := sec.DecodeKey("private_key"); err != nil {
		return fmt.Sprintf("%s has a certificate_chain and no private_key: the listener does not present it", naming.SecCert)
	} else if _, err := tls.X509KeyPair(b, key); err != nil {
		return fmt.Sprintf("%s certificate_chain and private_key are not a pair, the listener does not present them: %s", naming.SecCert, err)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Sprintf("%s certificate_chain: %s", naming.SecCert, err)
	}
	if err := cert.VerifyHostname(t.localhost); err != nil {
		return fmt.Sprintf("%s does not name %s, so the peers refuse its certificate: it names %s, set alt_names to hold {clusternodes}, or install one naming %s",
			naming.SecCert, t.localhost, certNames(cert), t.localhost)
	}
	return expiryIssue(naming.SecCert, cert, now, certExpiryWarning,
		"the peers refuse it from then", "the speaker renews it, unless it was installed from elsewhere: renew it with om system/sec/cert certificate renew")
}

// caIssue returns when the certificate of system/sec/ca expires, or expired,
// and "" when it is not soon to expire, or when the sec has no certificate to
// tell.
func caIssue(now time.Time) string {
	cert := secCertificate(naming.SecCa)
	if cert == nil {
		return ""
	}
	return expiryIssue(naming.SecCa, cert, now, caExpiryWarning,
		"the certificates it signed are refused from then, the ones of the nodes among them",
		"the speaker renews it with its key: renew it with om system/sec/ca certificate renew --force")
}

// trustedCAIssues returns when the certificates of the other authorities the
// cluster ca keyword lists expire, or expired, as a former ca a rotation kept
// for the certificates it signed: they are refused once it expires. Every
// certificate of their certificate_chain is one, as the listener trusts them
// all.
func trustedCAIssues(now time.Time) []string {
	cfg := cluster.ConfigData.Get()
	if cfg == nil {
		return nil
	}
	var issues []string
	for _, s := range cfg.CASecPaths {
		p, err := naming.ParsePath(s)
		if err != nil || p == naming.SecCa {
			continue
		}
		for _, cert := range secTrustedCertificates(p) {
			issue := expiryIssue(p, cert, now, caExpiryWarning,
				"the certificates it signed are refused from then",
				"renew them from system/sec/ca, and stop trusting it")
			if issue != "" && !slices.Contains(issues, issue) {
				issues = append(issues, issue)
			}
		}
	}
	return issues
}

// secTrustedCertificates returns the certificates of the certificate_chain
// of the sec at p, as the listener trusts them, or of its certificate when it
// has no chain.
func secTrustedCertificates(p naming.Path) []*x509.Certificate {
	if !p.Exists() {
		return nil
	}
	sec, err := object.NewSec(p, object.WithVolatile(true))
	if err != nil {
		return nil
	}
	b, err := sec.DecodeKey("certificate_chain")
	if err != nil {
		if b, err = sec.DecodeKey("certificate"); err != nil {
			return nil
		}
	}
	var l []*x509.Certificate
	for {
		var block *pem.Block
		if block, b = pem.Decode(b); block == nil {
			return l
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			l = append(l, cert)
		}
	}
}

// secCertificate returns the certificate of the sec at p, nil when it has
// none to tell.
func secCertificate(p naming.Path) *x509.Certificate {
	if !p.Exists() {
		return nil
	}
	sec, err := object.NewSec(p, object.WithVolatile(true))
	if err != nil {
		return nil
	}
	b, err := sec.DecodeKey("certificate")
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return cert
}

// expiryIssue returns the issue of a certificate of the sec at p expiring
// within warning, or expired, and "" when it is not.
func expiryIssue(p naming.Path, cert *x509.Certificate, now time.Time, warning time.Duration, consequence, remedy string) string {
	notAfter := cert.NotAfter.Local().Format(time.DateOnly)
	switch left := cert.NotAfter.Sub(now); {
	case left <= 0:
		return fmt.Sprintf("%s certificate expired on %s: %s, %s", p, notAfter, strings.Replace(consequence, "from then", "now", 1), remedy)
	case left <= warning:
		return fmt.Sprintf("%s certificate expires on %s: %s, %s", p, notAfter, consequence, remedy)
	default:
		return ""
	}
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
