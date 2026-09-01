package daemonauth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
)

type (
	// X509CACertFiler is the interface for X509CACertFile method for x509 auth.
	X509CACertFiler interface {
		X509CACertFile() string
	}

	X509Strategy struct {
		verifyOptions x509.VerifyOptions
		userDB        UserGranter
	}
)

// Authenticate verifies the client certificate against the cluster CA and
// reads the grants of the usr object named by its common name.
//
// The certificate is the credential and the common name is the identity:
// a certificate that verifies but names nobody this cluster knows is not
// authenticated.
func (s *X509Strategy) Authenticate(_ context.Context, r *http.Request) (*Info, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil, fmt.Errorf("strategies/x509: request has no client certificate")
	}
	opts := s.verifyOptions
	// The chain the client sent, minus its own certificate, is what the
	// verification may need to reach the CA.
	if len(r.TLS.PeerCertificates) > 1 {
		opts.Intermediates = x509.NewCertPool()
		for _, intermediate := range r.TLS.PeerCertificates[1:] {
			opts.Intermediates.AddCert(intermediate)
		}
	}
	chain, err := r.TLS.PeerCertificates[0].Verify(opts)
	if err != nil {
		return nil, fmt.Errorf("strategies/x509: %w", err)
	}
	username := chain[0][0].Subject.CommonName
	if username == "" {
		return nil, fmt.Errorf("strategies/x509: certificate subject has no common name")
	}
	key := cacheKey(StrategyX509, username)
	if info, ok := authCache.get(key); ok {
		return info, nil
	}
	grants, err := s.userDB.GrantsFromUsername(username)
	if err != nil {
		return nil, fmt.Errorf("strategies/x509: invalid user %s: %w", username, err)
	}
	info := &Info{
		Username: username,
		Strategy: StrategyX509,
		Grants:   grants,
	}
	authCache.set(key, info, cacheTTL)
	return info, nil
}

func initX509(_ context.Context, i interface{}) (string, Strategy, error) {
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
	userDB, ok := i.(UserGranter)
	if !ok {
		return name, nil, fmt.Errorf("UserGranter interface is not implemented")
	}
	return name, &X509Strategy{
		verifyOptions: x509.VerifyOptions{
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			Roots:     roots,
		},
		userDB: userDB,
	}, nil
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
