package object

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/fqdn"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
)

// OptsUnset is the options of the Unset object method.
type OptsGenCert struct {
	OptsLock
}

// GenCert generates a x509 certificate and adds (or replaces) it has a key set.
func (t *sec) GenCert(options OptsGenCert) error {
	var err error
	priv, err := t.getPriv()
	if err != nil {
		return err
	}
	ca := t.CertInfo("ca")
	switch ca {
	case "":
		err = t.genSelfSigned(priv)
	default:
		err = t.genCASigned(priv, ca)
	}
	if err != nil {
		return err
	}
	return t.config.Commit()
}

func CASecPaths() []path.T {
	ls := strings.Fields(rawconfig.ClusterSection().CASecPaths)
	l := make([]path.T, 0)
	for _, s := range ls {
		p, err := path.Parse(s)
		if err != nil {
			continue
		}
		l = append(l, p)
	}
	return l
}

func (t *sec) genSelfSigned(priv *rsa.PrivateKey) error {
	t.log.Debug().Msg("generate a self-signed certificate")
	tmpl, err := t.template(true, priv)
	if err != nil {
		return err
	}
	_, certBytes, err := genCert(&tmpl, &tmpl, priv)
	if err != nil {
		return err
	}
	return t.addKey("certificate", certBytes)
}

func (t *sec) genCASigned(priv *rsa.PrivateKey, ca string) error {
	t.log.Debug().Msgf("generate a certificate signed by the CA in %s", ca)
	caCert, caCertBytes, err := t.getCACert()
	if err != nil {
		return err
	}
	tmpl, err := t.template(false, priv)
	if err != nil {
		return err
	}
	_, certBytes, err := genCert(&tmpl, caCert, priv)
	if err != nil {
		return err
	}
	if err := t.addKey("certificate", certBytes); err != nil {
		return err
	}
	if err := t.addKey("certificate_chain", append(certBytes, caCertBytes...)); err != nil {
		return err
	}
	return nil
}

func (t *sec) CertInfo(name string) string {
	return t.config.GetString(key.Parse(name))
}

func (t *sec) CertInfoBits() int {
	sz := t.config.GetSize(key.Parse("bits"))
	return int(*sz)
}

func (t *sec) CertInfoNotAfter() (time.Time, error) {
	if v, err := t.config.GetDurationStrict(key.Parse("validity")); err != nil {
		return time.Now(), err
	} else {
		return time.Now().Add(*v), nil
	}
}

func (t *sec) IPAddressesFromAltNames() []net.IP {
	l := []net.IP{net.ParseIP("127.0.0.1")}
	for _, word := range t.config.GetSlice(key.Parse("alt_names")) {
		ip := net.ParseIP(word)
		if ip == nil {
			continue
		}
		l = append(l, ip)
	}
	return l
}

func (t *sec) DNSNamesFromAltNames() []string {
	l := []string{}
	for _, word := range t.config.GetSlice(key.Parse("alt_names")) {
		if !fqdn.IsValid(word) && !hostname.IsValid(word) {
			continue
		}
		l = append(l, word)
	}
	return l
}

func getBaseKeyUsage(priv interface{}) x509.KeyUsage {
	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}
	return keyUsage
}

func (t *sec) subject() pkix.Name {
	return pkix.Name{
		Country:            []string{t.CertInfo("c")},
		Organization:       []string{t.CertInfo("o")},
		OrganizationalUnit: []string{t.CertInfo("ou")},
		CommonName:         t.CertInfo("cn"),
	}
}

// "cn", "c", "st", "l", "o", "ou", "email", "alt_names", "bits", "validity", "ca"
func (t *sec) template(isCA bool, priv interface{}) (x509.Certificate, error) {
	keyUsage := getBaseKeyUsage(priv)
	notAfter, err := t.CertInfoNotAfter()
	if err != nil {
		return x509.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               t.subject(),
		NotBefore:             time.Now().Add(-10 * time.Second),
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		MaxPathLen:            2,
		IPAddresses:           t.IPAddressesFromAltNames(),
		DNSNames:              t.DNSNamesFromAltNames(),
	}
	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		template.KeyUsage |= x509.KeyUsageCRLSign
	}
	return template, nil
}

func (t *sec) getCASec() (*sec, error) {
	s := t.CertInfo("ca")
	p, err := path.Parse(s)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid ca secret path: %s", s)
	}
	if !Exists(p) {
		return nil, fmt.Errorf("secret %s does not exist", p.String())
	}
	return NewSec(p, WithVolatile(true))
}

func (t *sec) setPriv(priv *rsa.PrivateKey) error {
	b, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b})
	return t.addKey("private_key", pemBytes)
}

func (t *sec) setCert(derBytes []byte) error {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return t.addKey("certificate", pemBytes)
}

func (t *sec) getCACert() (*x509.Certificate, []byte, error) {
	var (
		sec  *sec
		b    []byte
		err  error
		cert *x509.Certificate
	)
	if sec, err = t.getCASec(); err != nil {
		return nil, nil, err
	}
	if b, err = sec.decode("certificate"); err != nil {
		return nil, nil, err
	}
	if cert, err = certFromPEM(b); err != nil {
		return nil, nil, err
	}
	return cert, b, nil
}

func certFromPEM(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("certFromPEM: PEM block type is not CERTIFICATE")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certFromPEM: failed to parse certificate: " + err.Error())
	}
	return cert, nil
}

func (t *sec) getCAPriv() (*rsa.PrivateKey, error) {
	var (
		sec *sec
		b   []byte
		err error
	)
	if sec, err = t.getCASec(); err != nil {
		return nil, err
	}
	if b, err = sec.decode("private_key"); err != nil {
		return nil, err
	}
	return privFromPEM(b)
}

func privFromPEM(b []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(b)
	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("PEM block type is not PRIVATE KEY")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return priv.(*rsa.PrivateKey), nil
}

func (t *sec) getPriv() (*rsa.PrivateKey, error) {
	b, err := t.decode("private_key")
	if err != nil {
		return t.genPriv()
	}
	priv, err := privFromPEM(b)
	if err != nil {
		return t.genPriv()
	}
	return priv, nil
}

func (t *sec) genPriv() (*rsa.PrivateKey, error) {
	bits := t.CertInfoBits()
	t.log.Info().Int("bits", bits).Msg("generate new private key")
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	if err := t.setPriv(priv); err != nil {
		return nil, err
	}
	return priv, nil
}

func genCert(template, parent *x509.Certificate, priv *rsa.PrivateKey) (*x509.Certificate, []byte, error) {
	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: " + err.Error())
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: " + err.Error())
	}

	b := pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	certPEM := pem.EncodeToMemory(&b)

	return cert, certPEM, nil
}
