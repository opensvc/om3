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

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
)

// OptsUnset is the options of the Unset object method.
type OptsGenCert struct {
	Global OptsGlobal
	Lock   OptsLocking
}

// GenCert generates a x509 certificate and adds (or replaces) it has a key set.
func (t *Sec) GenCert(options OptsGenCert) error {
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

func (t *Sec) genSelfSigned(priv *rsa.PrivateKey) error {
	t.log.Info().Msg("generate a self-signed certificate")
	tmpl, err := t.template(true, priv)
	if err != nil {
		return err
	}
	_, pemBytes, err := genCert(&tmpl, &tmpl, priv)
	if err != nil {
		return err
	}
	return t.addKey("certificate", pemBytes, t)
}

func (t *Sec) genCASigned(priv *rsa.PrivateKey, ca string) error {
	t.log.Info().Msgf("generate a certificate signed by the CA in %s", ca)
	return nil
}

func (t *Sec) CertInfo(name string) string {
	return t.config.GetString(key.Parse(name))
}

func (t *Sec) CertInfoBits() int {
	sz := t.config.GetSize(key.Parse("bits"))
	return int(*sz)
}

func (t *Sec) CertInfoNotAfter() (time.Time, error) {
	if v, err := t.config.GetDurationStrict(key.Parse("validity")); err != nil {
		return time.Now(), err
	} else {
		return time.Now().Add(*v), nil
	}
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

// "cn", "c", "st", "l", "o", "ou", "email", "alt_names", "bits", "validity", "ca"
func (t *Sec) template(isCA bool, priv interface{}) (x509.Certificate, error) {
	keyUsage := getBaseKeyUsage(priv)
	notAfter, err := t.CertInfoNotAfter()
	if err != nil {
		return x509.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Country:            []string{t.CertInfo("c")},
			Organization:       []string{t.CertInfo("o")},
			OrganizationalUnit: []string{t.CertInfo("ou")},
			CommonName:         t.CertInfo("cn"),
		},
		NotBefore:             time.Now().Add(-10 * time.Second),
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		MaxPathLen:            2,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		template.KeyUsage |= x509.KeyUsageCRLSign
	}
	return template, nil
}

func (t *Sec) getCASec() (*Sec, error) {
	s := t.CertInfo("ca")
	p, err := path.Parse(s)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid ca secret path: %s", s)
	}
	sec := NewSec(p, WithVolatile(true))
	if !sec.Exists() {
		return sec, fmt.Errorf("secret %s does not exist")
	}
	return sec, nil
}

func (t *Sec) setPriv(priv *rsa.PrivateKey) error {
	b, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b})
	return t.addKey("private_key", pemBytes, t)
}

func (t *Sec) setCert(derBytes []byte) error {
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return t.addKey("certificate", pemBytes, t)
}

func (t *Sec) getCAPriv() (*rsa.PrivateKey, error) {
	var (
		sec *Sec
		b   []byte
		err error
	)
	if sec, err = t.getCASec(); err != nil {
		return nil, err
	}
	if b, err = sec.decode("private_key", t); err != nil {
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

func (t *Sec) getPriv() (*rsa.PrivateKey, error) {
	b, err := t.decode("private_key", t)
	if err != nil {
		return t.genPriv()
	}
	priv, err := privFromPEM(b)
	if err != nil {
		return t.genPriv()
	}
	return priv, nil
}

func (t *Sec) genPriv() (*rsa.PrivateKey, error) {
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

func (t *Sec) genCA() (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	priv, err := t.getPriv()
	if err != nil {
		return nil, nil, nil, err
	}
	rootTemplate, err := t.template(true, priv)
	if err != nil {
		return nil, nil, nil, err
	}
	rootCert, rootPEM, err := genCert(&rootTemplate, &rootTemplate, priv)
	if err != nil {
		return nil, nil, nil, err
	}
	return rootCert, rootPEM, priv, nil
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
