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
	"time"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

// GenCert generates a x509 certificate and adds (or replaces) it has a key set.
func (t *sec) GenCert() error {
	var err error
	ca := t.CertInfo("ca")
	switch ca {
	case "":
		err = t.genSelfSigned()
	default:
		err = t.genCASigned(ca)
	}
	if err != nil {
		return err
	}
	return t.config.Commit()
}

func (t *sec) genSelfSigned() error {
	t.log.Tracef("generate a self-signed certificate")
	priv, err := t.getPriv()
	if err != nil {
		return err
	}
	tmpl, err := t.template(true, priv)
	if err != nil {
		return err
	}
	_, certBytes, err := genCert(&tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	privBytes, err := t.privPEM(priv)
	if err != nil {
		return err
	}
	if err := t.addKey("certificate", certBytes); err != nil {
		return err
	}
	if err := t.addKey("certificate_chain", certBytes); err != nil {
		return err
	}
	if err := t.addKey("fullpem", append(privBytes, certBytes...)); err != nil {
		return err
	}
	if err := t.addKey("serial_number", []byte(tmpl.SerialNumber.String())); err != nil {
		return err
	}
	return nil
}

func (t *sec) genCASigned(ca string) error {
	t.log.Tracef("generate a certificate signed by the %s CA", ca)
	priv, err := t.getPriv()
	if err != nil {
		return err
	}
	caCert, caCertBytes, err := t.getCACert()
	if err != nil {
		return err
	}
	caPriv, err := t.getCAPriv()
	if err != nil {
		return err
	}
	tmpl, err := t.template(false, priv)
	if err != nil {
		return err
	}
	_, certBytes, err := genCert(&tmpl, caCert, &priv.PublicKey, caPriv)
	if err != nil {
		return err
	}
	privBytes, err := t.privPEM(priv)
	if err != nil {
		return err
	}
	chainBytes := append(certBytes, caCertBytes...)
	if err := t.addKey("certificate", certBytes); err != nil {
		return err
	}
	if err := t.addKey("certificate_chain", chainBytes); err != nil {
		return err
	}
	if err := t.addKey("fullpem", append(privBytes, chainBytes...)); err != nil {
		return err
	}
	if err := t.addKey("serial_number", []byte(tmpl.SerialNumber.String())); err != nil {
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

func (t *sec) CertSerial() *big.Int {
	bi := big.NewInt(int64(0))
	if b, err := t.DecodeKey("serial_number"); err != nil {
		return bi
	} else if v, ok := bi.SetString(string(b), 10); ok && v != nil {
		return v
	} else {
		return bi
	}
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
	for _, word := range t.config.GetStrings(key.Parse("alt_names")) {
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
	for _, word := range t.config.GetStrings(key.Parse("alt_names")) {
		if !naming.IsValidFQDN(word) && !hostname.IsValid(word) {
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
	inc := big.NewInt(1)
	serial := t.CertSerial()
	serial = serial.Add(serial, inc)
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               t.subject(),
		NotBefore:             time.Now().Add(-10 * time.Second),
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           t.IPAddressesFromAltNames(),
		DNSNames:              t.DNSNamesFromAltNames(),
	}
	if isCA {
		template.IsCA = true
		template.MaxPathLen = 2
		template.KeyUsage |= x509.KeyUsageCertSign
		template.KeyUsage |= x509.KeyUsageCRLSign
		template.ExtKeyUsage = []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		}
	}
	if t.path.Kind == naming.KindUsr {
		template.ExtKeyUsage = []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		}
	}
	return template, nil
}

func (t *sec) getCASec() (*sec, error) {
	s := t.CertInfo("ca")
	p, err := naming.ParsePath(s)
	if err != nil {
		return nil, fmt.Errorf("invalid ca secret path %s: %w", s, err)
	}
	if !p.Exists() {
		return nil, fmt.Errorf("secret %s does not exist", p.String())
	}
	return NewSec(p, WithVolatile(true))
}

func (t *sec) privPEM(priv *rsa.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return []byte{}, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b})
	return pemBytes, nil
}

func (t *sec) setPriv(priv *rsa.PrivateKey) error {
	pemBytes, err := t.privPEM(priv)
	if err != nil {
		return err
	}
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
		return nil, fmt.Errorf("certFromPEM: the PEM block type is not CERTIFICATE")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certFromPEM: failed to parse certificate: %w", err)
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
		return nil, fmt.Errorf("privFromPEM: the PEM block type is not PRIVATE KEY")
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
	t.log.Infof("generate a new %d bits private key", bits)
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	if err := t.setPriv(priv); err != nil {
		return nil, err
	}
	return priv, nil
}

func genCert(template, parent *x509.Certificate, pub interface{}, priv *rsa.PrivateKey) (*x509.Certificate, []byte, error) {
	certBytes, err := x509.CreateCertificate(rand.Reader, template, parent, pub, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	b := pem.Block{Type: "CERTIFICATE", Bytes: certBytes}
	certPEM := pem.EncodeToMemory(&b)

	return cert, certPEM, nil
}
