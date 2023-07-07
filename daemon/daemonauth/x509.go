package daemonauth

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/shaj13/go-guardian/v2/auth"
	x509Strategy "github.com/shaj13/go-guardian/v2/auth/strategies/x509"
)

type (
	// X509CACertFiler is the interface for X509CACertFile method for x509 auth.
	X509CACertFiler interface {
		X509CACertFile() string
	}
)

func initX509(i interface{}) (string, auth.Strategy, error) {
	name := "x509"
	caFiler, ok := i.(X509CACertFiler)
	if !ok {
		return name, nil, fmt.Errorf("missing ca certificates")
	}
	caCertsFile := caFiler.X509CACertFile()
	cert, err := x509CertificateFromFile(caCertsFile)
	if err != nil {
		return name, nil, fmt.Errorf("initX509 retrieve cert from file %s: %w", caCertsFile, err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	opts := x509.VerifyOptions{
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Roots:     roots,
	}
	return name, x509Strategy.New(opts), nil
}

func x509CertificateFromFile(s string) (*x509.Certificate, error) {
	ca, err := os.ReadFile(s)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	p, _ := pem.Decode(ca)
	if p == nil {
		return nil, fmt.Errorf("pem decode: %w", err)
	}
	cert, err := x509.ParseCertificate(p.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509 parse certificate: %w", err)
	}
	return cert, nil
}
