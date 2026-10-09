package object

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	stdlog "log"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	legolog "github.com/go-acme/lego/v4/log"
	"github.com/go-acme/lego/v4/providers/http/webroot"
	"github.com/go-acme/lego/v4/registration"

	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// CertificateRenewal tells what a certificate renewal did.
	CertificateRenewal struct {
		// Renewed is true when a certificate was issued.
		Renewed bool `json:"renewed"`

		// Reason says why a certificate was issued, or why none was.
		Reason string `json:"reason"`

		// Sec is the sec the certificate is of, for a renewal of several.
		Sec string `json:"sec,omitempty"`

		Domains []string `json:"domains"`

		// Directory is the ACME directory the certificate is issued by,
		// empty for a certificate generated, self-signed or signed by the
		// ca of the sec.
		Directory string    `json:"directory,omitempty"`
		NotAfter  time.Time `json:"not_after,omitzero"`
	}

	// CertificateRenewOptions says how a certificate is renewed.
	CertificateRenewOptions struct {
		// Force issues a certificate even when the current one is not due.
		Force bool

		// Webroot is the host directory the http-01 challenge token is
		// written in, acme.webroot when empty. It is ignored when HTTP01
		// is set.
		Webroot string

		// HTTP01 writes the http-01 challenge token where the http server
		// of the domains serves it from, as a task writes it in a volume of
		// its object, confined there.
		HTTP01 challenge.Provider
	}

	// acmeAccount is what the sec keeps of its account at the ACME
	// directory, besides the account key: an account is valid at the
	// directory it was registered with only.
	acmeAccount struct {
		Directory    string                 `json:"directory"`
		Email        string                 `json:"email"`
		Registration *registration.Resource `json:"registration"`
	}

	// acmeUser is the account as the lego client reads it.
	acmeUser struct {
		email string
		reg   *registration.Resource
		key   crypto.PrivateKey
	}
)

const (
	// AcmeLetsEncrypt is the directory of Let's Encrypt, which
	// acme.directory = letsencrypt names.
	AcmeLetsEncrypt = "https://acme-v02.api.letsencrypt.org/directory"

	// AcmeLetsEncryptStaging is the staging directory of Let's Encrypt,
	// which acme.directory = letsencrypt-staging names.
	AcmeLetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"

	acmeDefaultRenewBefore = 30 * 24 * time.Hour

	keyAcmeAccountKey = "acme_account_key"
	keyAcmeAccount    = "acme_account"

	// keyAcmeIssuedBy is the directory the current certificate was issued
	// by: a certificate of another directory than the sec asks, as one of
	// the staging directory a setup was tried out with, is not the one
	// asked.
	keyAcmeIssuedBy = "acme_issued_by"
)

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.reg }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// Render is the renewal as a person reads it.
func (t CertificateRenewal) Render() string {
	var b strings.Builder
	if t.Renewed {
		fmt.Fprintf(&b, "certificate issued: %s\n", t.Reason)
	} else {
		fmt.Fprintf(&b, "no certificate issued: %s\n", t.Reason)
	}
	if t.Sec != "" {
		fmt.Fprintf(&b, "sec        %s\n", t.Sec)
	}
	fmt.Fprintf(&b, "domains    %s\n", strings.Join(t.Domains, " "))
	if t.Directory != "" {
		fmt.Fprintf(&b, "directory  %s\n", t.Directory)
	} else {
		fmt.Fprintf(&b, "issuer     the sec, self-signed or signed by its ca\n")
	}
	if !t.NotAfter.IsZero() {
		fmt.Fprintf(&b, "expires    %s\n", t.NotAfter.Local().Truncate(time.Second).Format(time.RFC3339))
	}
	return b.String()
}

// acmeDomains returns the domains the certificate is issued for: the common
// name, then the alternate names, the common name being the subject of the
// certificate.
func (t *sec) acmeDomains() []string {
	l := make([]string, 0)
	if cn := t.config.GetString(key.Parse("cn")); cn != "" {
		l = append(l, cn)
	}
	for _, name := range t.config.GetStrings(key.Parse("alt_names")) {
		if !slices.Contains(l, name) {
			l = append(l, name)
		}
	}
	return l
}

// acmeRenewalDue says whether a certificate is to be issued for domains, and
// why: there is none, it is not one of an ACME directory, it does not name
// the domains asked, or it expires within before.
func acmeRenewalDue(certPEM []byte, domains []string, now time.Time, before time.Duration) (bool, string) {
	return certificateDue(certPEM, domains, now, before, true)
}

// certificateDue says whether a certificate is to be issued for domains, and
// why: there is none, it does not name the domains asked, it expires within
// before, or it is self-signed where a certificate of an issuer is asked.
func certificateDue(certPEM []byte, domains []string, now time.Time, before time.Duration, selfSignedIsDue bool) (bool, string) {
	if len(certPEM) == 0 {
		return true, "no certificate"
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return true, "the certificate is not readable"
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, "the certificate is not readable: " + err.Error()
	}
	if selfSignedIsDue && cert.Issuer.String() == cert.Subject.String() {
		return true, "the certificate is self-signed"
	}
	have := slices.Clone(cert.DNSNames)
	want := slices.Clone(domains)
	slices.Sort(have)
	slices.Sort(want)
	if !slices.Equal(have, want) {
		return true, fmt.Sprintf("the certificate names %s, the sec asks %s", strings.Join(have, " "), strings.Join(want, " "))
	}
	if left := cert.NotAfter.Sub(now); left <= before {
		return true, fmt.Sprintf("the certificate expires on %s, within %s", cert.NotAfter.Local().Format(time.RFC3339), before)
	}
	return false, fmt.Sprintf("the certificate is valid until %s", cert.NotAfter.Local().Format(time.RFC3339))
}

// certificateIPsDue says whether certPEM is due because its ip addresses are
// not ips, which a generated certificate is issued for as alt_names lists
// them, and returns reason as is when it is not.
func certificateIPsDue(certPEM []byte, ips []net.IP, reason string) (bool, string) {
	cert, err := certFromPEM(certPEM)
	if err != nil {
		return true, "the certificate is not readable: " + err.Error()
	}
	text := func(l []net.IP) []string {
		s := make([]string, len(l))
		for i, ip := range l {
			s[i] = ip.String()
		}
		slices.Sort(s)
		return slices.Compact(s)
	}
	have, want := text(cert.IPAddresses), text(ips)
	if !slices.Equal(have, want) {
		return true, fmt.Sprintf("the certificate ip addresses are %s, the sec asks %s", strings.Join(have, " "), strings.Join(want, " "))
	}
	return false, reason
}

// RenewCertificate issues the certificate of the sec from its ACME
// directory, when one is due, and stores it as the private_key, certificate,
// certificate_chain and fullpem keys, as a certificate create does. The keys
// installed in volumes are installed anew there, and the signals of their
// install lines sent.
//
// The domain is proved with the http-01 challenge: the token is written in
// webroot, under .well-known/acme-challenge, where the http server of the
// domain must serve it from.
//
// The account at the directory is registered on the first renewal, and kept
// as keys of the sec, so the node renewing next, whichever it is, renews
// with the same account.
func (t *sec) RenewCertificate(ctx context.Context, opts CertificateRenewOptions) (CertificateRenewal, error) {
	directory := acmeDirectory(t.config.GetString(key.Parse("acme.directory")))
	if directory == "" {
		return t.renewGeneratedCertificate(opts.Force)
	}
	force, webrootPath := opts.Force, opts.Webroot
	result := CertificateRenewal{Domains: t.acmeDomains(), Directory: directory}
	if len(result.Domains) == 0 {
		return result, fmt.Errorf("no domain to issue a certificate for: set cn, and alt_names for more")
	}
	// The email is the contact of the account, which a directory may
	// write to about it: an account without one is valid all the same.
	email := t.config.GetString(key.Parse("email"))
	if webrootPath == "" {
		webrootPath = t.config.GetString(key.Parse("acme.webroot"))
	}
	if opts.HTTP01 == nil && webrootPath == "" {
		// A sec whose certificate the listener presents is proved through
		// the listener, which every node answers the token on.
		if provider, err := t.newListenerHTTP01(); err != nil {
			return result, err
		} else if provider != nil {
			opts.HTTP01 = provider
		} else {
			return result, fmt.Errorf("no webroot to prove the domains from: set acme.webroot, the directory the http server of the domains serves /.well-known/acme-challenge/ from, or list %s in listener.tls_secs for the listener to answer the challenge", t.path)
		}
	}
	before := acmeDefaultRenewBefore
	if d := t.config.GetDuration(key.Parse("acme.renew_before")); d != nil {
		before = *d
	}

	current, _ := t.decode("certificate")
	due, reason := acmeRenewalDue(current, result.Domains, time.Now(), before)
	if issuedBy, _ := t.decode(keyAcmeIssuedBy); !due && len(issuedBy) > 0 && string(issuedBy) != directory {
		due, reason = true, fmt.Sprintf("the certificate was issued by %s, the sec asks %s", issuedBy, directory)
	}
	switch {
	case force:
		result.Reason = "asked"
	case !due:
		result.Reason = reason
		return result, nil
	default:
		result.Reason = reason
	}

	user, err := t.acmeUser(directory, email)
	if err != nil {
		return result, err
	}
	// The progress of the client is what the log of the sec says, at
	// debug level: the result and the errors are what the command says.
	legolog.Logger = stdlog.New(debugWriter{t.log}, "", 0)
	cfg := lego.NewConfig(user)
	cfg.CADirURL = directory
	cfg.Certificate.KeyType = certcrypto.RSA2048
	client, err := lego.NewClient(cfg)
	if err != nil {
		return result, fmt.Errorf("acme client of %s: %w", directory, err)
	}
	provider := opts.HTTP01
	if provider == nil {
		if provider, err = webroot.NewHTTPProvider(webrootPath); err != nil {
			return result, fmt.Errorf("acme webroot %s: %w", webrootPath, err)
		}
	}
	if err := client.Challenge.SetHTTP01Provider(provider); err != nil {
		return result, err
	}
	if user.reg == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return result, fmt.Errorf("register an account at %s: %w", directory, err)
		}
		user.reg = reg
		if err := t.saveAcmeAccount(acmeAccount{Directory: directory, Email: email, Registration: reg}); err != nil {
			return result, err
		}
	}
	res, err := client.Certificate.Obtain(certificate.ObtainRequest{Domains: result.Domains, Bundle: true})
	if err != nil {
		// The account is kept, so the next try does not register another.
		_ = t.config.Commit()
		return result, fmt.Errorf("obtain a certificate for %s: %w", strings.Join(result.Domains, " "), err)
	}
	if err := t.storeAcmeCertificate(res); err != nil {
		return result, err
	}
	if err := t.addKey(keyAcmeIssuedBy, []byte(directory)); err != nil {
		return result, err
	}
	if block, _ := pem.Decode(res.Certificate); block != nil {
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			result.NotAfter = cert.NotAfter
		}
	}
	result.Renewed = true
	return result, t.config.Commit()
}

// acmeUser returns the account of the sec at the directory, its key made
// when there is none. The registration is nil when the account is not
// registered at this directory yet.
func (t *sec) acmeUser(directory, email string) (*acmeUser, error) {
	user := &acmeUser{email: email}
	if b, err := t.decode(keyAcmeAccountKey); err == nil && len(b) > 0 {
		priv, err := certcrypto.ParsePEMPrivateKey(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", keyAcmeAccountKey, err)
		}
		user.key = priv
	} else {
		priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		if err := t.addKey(keyAcmeAccountKey, certcrypto.PEMEncode(priv)); err != nil {
			return nil, err
		}
		user.key = priv
	}
	if b, err := t.decode(keyAcmeAccount); err == nil && len(b) > 0 {
		var account acmeAccount
		if err := json.Unmarshal(b, &account); err != nil {
			return nil, fmt.Errorf("%s: %w", keyAcmeAccount, err)
		}
		// An account registered at another directory, or for another
		// email, is not the one asked: it is registered anew.
		if account.Directory == directory && account.Email == email {
			user.reg = account.Registration
		}
	}
	return user, nil
}

func (t *sec) saveAcmeAccount(account acmeAccount) error {
	b, err := json.Marshal(account)
	if err != nil {
		return err
	}
	return t.addKey(keyAcmeAccount, b)
}

// storeAcmeCertificate stores an issued certificate as the keys a
// certificate create writes: the private key in PKCS#8, the certificate, the
// chain, and the private key followed by the chain, which a proxy like
// haproxy reads.
func (t *sec) storeAcmeCertificate(res *certificate.Resource) error {
	if res == nil || len(res.Certificate) == 0 || len(res.PrivateKey) == 0 {
		return errors.New("the acme directory returned no certificate")
	}
	priv, err := certcrypto.ParsePEMPrivateKey(res.PrivateKey)
	if err != nil {
		return fmt.Errorf("the private key of the certificate: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	chain := res.Certificate
	leaf := chain
	if block, _ := pem.Decode(chain); block != nil {
		leaf = pem.EncodeToMemory(block)
	}
	for k, v := range map[string][]byte{
		"private_key":       privPEM,
		"certificate":       leaf,
		"certificate_chain": chain,
		"fullpem":           append(append([]byte{}, privPEM...), chain...),
	} {
		if err := t.addKey(k, v); err != nil {
			return err
		}
	}
	return nil
}

// debugWriter writes the lines of a standard logger to a logger, at debug
// level.
type debugWriter struct {
	log *plog.Logger
}

func (w debugWriter) Write(b []byte) (int, error) {
	w.log.Debugf("acme: %s", strings.TrimRight(string(b), "\n"))
	return len(b), nil
}

// acmeDirectory returns the url of the ACME directory acme.directory names,
// its aliases resolved, and empty when it names none.
func acmeDirectory(s string) string {
	switch s {
	case "letsencrypt":
		return AcmeLetsEncrypt
	case "letsencrypt-staging":
		return AcmeLetsEncryptStaging
	default:
		return s
	}
}

// renewGeneratedCertificate generates the certificate of a sec issued by no
// ACME directory, as certificate create does, when it is due: self-signed,
// or signed by the ca the sec names. The private key is kept.
// ErrCertificateNotIssued is the refusal to renew a certificate the sec did
// not issue, as one installed from elsewhere: generating one would replace it,
// with the names of the sec rather than the ones it was installed for.
var ErrCertificateNotIssued = errors.New("the certificate was not issued by the sec")

// issuedCertificate says whether certPEM is a certificate the sec issued: one
// its ca signed, or a self-signed one when it names no ca.
func (t *sec) issuedCertificate(certPEM []byte) (bool, error) {
	cert, err := certFromPEM(certPEM)
	if err != nil {
		return false, err
	}
	if t.CertInfo("ca") == "" {
		return cert.CheckSignatureFrom(cert) == nil, nil
	}
	caCert, _, err := t.getCACert()
	if err != nil {
		return false, err
	}
	return cert.CheckSignatureFrom(caCert) == nil, nil
}

func (t *sec) renewGeneratedCertificate(force bool) (CertificateRenewal, error) {
	// The names the certificate is issued for, as the result says them:
	// the subject first. The alternate names are what it is compared
	// with, the common name not being one of them.
	result := CertificateRenewal{Domains: t.acmeDomains()}
	before := acmeDefaultRenewBefore
	if d := t.config.GetDuration(key.Parse("acme.renew_before")); d != nil {
		before = *d
	}
	current, _ := t.decode("certificate")
	due, reason := certificateDue(current, t.DNSNamesFromAltNames(), time.Now(), before, false)
	if !due {
		due, reason = certificateIPsDue(current, t.IPAddressesFromAltNames(), reason)
	}
	switch {
	case force:
		result.Reason = "asked"
	case !due:
		result.Reason = reason
		return result, nil
	default:
		result.Reason = reason
		// A certificate installed from elsewhere is due as soon as its
		// names are not alt_names, which they never are: renewing it
		// would replace it, so it is renewed when asked only.
		if len(current) > 0 {
			if issued, err := t.issuedCertificate(current); err != nil {
				return result, err
			} else if !issued {
				return result, fmt.Errorf("%w, it is kept: %s, renew it with --force to replace it with one the sec issues", ErrCertificateNotIssued, reason)
			}
		}
	}
	if err := t.GenCert(); err != nil {
		return result, err
	}
	if b, err := t.decode("certificate"); err == nil {
		if block, _ := pem.Decode(b); block != nil {
			if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				result.NotAfter = cert.NotAfter
			}
		}
	}
	result.Renewed = true
	return result, nil
}
