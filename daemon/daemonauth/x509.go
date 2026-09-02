package daemonauth

import (
	"context"
	"crypto/x509"
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
	roots, err := x509CertPoolFromFile(caCertsFile)
	if err != nil {
		return name, nil, fmt.Errorf("initX509 retrieve cert from file %s: %w", caCertsFile, err)
	}
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

// x509CertPoolFromFile returns the authorities a client certificate may
// be signed by.
//
// The file holds the certificate chain of every sec of the cluster ca
// keyword, whose documented promise is that a client certificate trusted
// by any CA certificate found in them is accepted. Only the first
// certificate of the file used to be read, so the promise held for one
// authority and the others were silently ignored.
func x509CertPoolFromFile(s string) (*x509.CertPool, error) {
	b, err := os.ReadFile(s)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(b) {
		return nil, fmt.Errorf("no certificate in %s", s)
	}
	return pool, nil
}
