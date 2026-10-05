// Package tlssecs reads the secs of listener.tls_secs for the listeners: the
// certificates the inet listener presents to the clients asking one of their
// names, and the http-01 challenge tokens the acme listener answers.
package tlssecs

import (
	"crypto/tls"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Store holds the certificates of the secs, each read again when the
	// configuration of its sec changes, as a renewal does.
	Store struct {
		log *plog.Logger

		mu    sync.Mutex
		certs map[naming.Path]certEntry
	}

	certEntry struct {
		modTime time.Time
		cert    *tls.Certificate
	}
)

func New(log *plog.Logger) *Store {
	return &Store{log: log, certs: make(map[naming.Path]certEntry)}
}

// Paths returns the secs listener.tls_secs names.
func Paths() []naming.Path {
	cfg := cluster.ConfigData.Get()
	if cfg == nil {
		return nil
	}
	return object.ListenerTLSSecs(cfg.Listener.TLSSecs)
}

// GetCertificate returns the certificate of the first sec of
// listener.tls_secs naming the server the client asks for, and none for the
// listener to present its own, the one of system/sec/cert.
func (t *Store) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		return nil, nil
	}
	for _, p := range Paths() {
		cert := t.certificate(p)
		if cert == nil {
			continue
		}
		if err := hello.SupportsCertificate(cert); err == nil {
			return cert, nil
		}
	}
	return nil, nil
}

// certificate returns the certificate of the sec, read again when its
// configuration changed since, and nil when it has none.
func (t *Store) certificate(p naming.Path) *tls.Certificate {
	info, err := os.Stat(p.ConfigFile())
	if err != nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.certs[p]; ok && e.modTime.Equal(info.ModTime()) {
		return e.cert
	}
	// A sec without a certificate, or with a bad one, is cached as such
	// too, so a handshake does not read it again until it changes.
	e := certEntry{modTime: info.ModTime()}
	if cert, err := load(p); errors.Is(err, object.ErrKeyNotExist) {
		// A sec whose first certificate is not obtained yet.
		t.log.Debugf("listener.tls_secs: %s: no certificate yet", p)
	} else if err != nil {
		t.log.Warnf("listener.tls_secs: %s: %s", p, err)
	} else {
		e.cert = cert
		t.log.Infof("listener.tls_secs: %s: certificate for %v loaded", p, cert.Leaf.DNSNames)
	}
	t.certs[p] = e
	return e.cert
}

func load(p naming.Path) (*tls.Certificate, error) {
	sec, err := object.NewSec(p, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	l, err := sec.DecodeKeys("certificate_chain", "private_key")
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(l[0], l[1])
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// KeyAuthorization returns the key authorization of the http-01 challenge
// token a renewal stored in a sec of listener.tls_secs, and false when no
// such sec holds it.
func KeyAuthorization(token string) ([]byte, bool) {
	k, err := object.AcmeChallengeKey(token)
	if err != nil {
		return nil, false
	}
	for _, p := range Paths() {
		if !p.Exists() {
			continue
		}
		sec, err := object.NewSec(p, object.WithVolatile(true))
		if err != nil || !sec.HasKey(k) {
			continue
		}
		if b, err := sec.DecodeKey(k); err == nil {
			return b, true
		}
	}
	return nil, false
}
