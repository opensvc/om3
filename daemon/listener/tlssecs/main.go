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
	// Store holds the certificates of the secs, and their challenge
	// tokens, each read again when the configuration of its sec changes,
	// as a renewal does.
	Store struct {
		log *plog.Logger

		mu     sync.Mutex
		certs  map[naming.Path]certEntry
		tokens map[naming.Path]tokensEntry

		// certFile and keyFile are the files of the certificate the
		// listener presents when no sec names the server the client
		// asks for, the one of system/sec/cert.
		certFile, keyFile string
		defaultCert       defaultCertEntry
	}

	// defaultCertEntry is the certificate of the files, as read when they
	// had these modification times.
	defaultCertEntry struct {
		certModTime, keyModTime time.Time
		cert                    *tls.Certificate
	}

	certEntry struct {
		modTime time.Time
		cert    *tls.Certificate
	}

	// tokensEntry is the key authorizations of the challenge tokens of a
	// sec, by key.
	tokensEntry struct {
		modTime time.Time
		keyAuth map[string][]byte
	}
)

func New(log *plog.Logger) *Store {
	return &Store{
		log:    log,
		certs:  make(map[naming.Path]certEntry),
		tokens: make(map[naming.Path]tokensEntry),
	}
}

// Paths returns the secs listener.tls_secs names.
func Paths() []naming.Path {
	cfg := cluster.ConfigData.Get()
	if cfg == nil {
		return nil
	}
	return object.ListenerTLSSecs(cfg.Listener.TLSSecs)
}

// WithDefault sets the files of the certificate GetCertificate returns when
// no sec of listener.tls_secs names the server the client asks for.
func (t *Store) WithDefault(certFile, keyFile string) *Store {
	t.certFile, t.keyFile = certFile, keyFile
	return t
}

// GetCertificate returns the certificate of the first sec of
// listener.tls_secs naming the server the client asks for, else the one of
// the default files, the one of system/sec/cert.
//
// The default files are read again when they change, as when system/sec/cert
// is renewed. The listener has no certificate of its own: the clients asking
// no name, as by an ip address, are presented the one of the files too.
func (t *Store) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		return t.defaultCertificate(), nil
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
	return t.defaultCertificate(), nil
}

// defaultCertificate returns the certificate of the default files, read again
// when one of them changed since, and nil when they are not set or can not be
// read.
//
// The two files are written one after the other, so a handshake between the
// two writes reads a key that is not the one of the certificate: that pair is
// not kept, and the next handshake reads them again.
func (t *Store) defaultCertificate() *tls.Certificate {
	if t.certFile == "" || t.keyFile == "" {
		return nil
	}
	certInfo, err := os.Stat(t.certFile)
	if err != nil {
		return nil
	}
	keyInfo, err := os.Stat(t.keyFile)
	if err != nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e := t.defaultCert
	if e.cert != nil && e.certModTime.Equal(certInfo.ModTime()) && e.keyModTime.Equal(keyInfo.ModTime()) {
		return e.cert
	}
	cert, err := tls.LoadX509KeyPair(t.certFile, t.keyFile)
	if err != nil {
		t.log.Debugf("listener: read %s and %s: %s", t.certFile, t.keyFile, err)
		return e.cert
	}
	t.defaultCert = defaultCertEntry{certModTime: certInfo.ModTime(), keyModTime: keyInfo.ModTime(), cert: &cert}
	if e.cert != nil {
		t.log.Infof("listener: certificate of %s loaded again", t.certFile)
	}
	return &cert
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
//
// The tokens of a sec are read when it changes only: a request for a token
// costs a stat of each sec, however many an internet client sends.
func (t *Store) KeyAuthorization(token string) ([]byte, bool) {
	k, err := object.AcmeChallengeKey(token)
	if err != nil {
		return nil, false
	}
	for _, p := range Paths() {
		if b, ok := t.challengeTokens(p)[k]; ok {
			return b, true
		}
	}
	return nil, false
}

// challengeTokens returns the key authorizations of the challenge tokens the
// sec holds, read again when its configuration changed since.
func (t *Store) challengeTokens(p naming.Path) map[string][]byte {
	info, err := os.Stat(p.ConfigFile())
	if err != nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.tokens[p]; ok && e.modTime.Equal(info.ModTime()) {
		return e.keyAuth
	}
	e := tokensEntry{modTime: info.ModTime(), keyAuth: make(map[string][]byte)}
	if sec, err := object.NewSec(p, object.WithVolatile(true)); err != nil {
		t.log.Warnf("listener.tls_secs: %s: %s", p, err)
	} else if keys, err := sec.MatchingKeys(object.AcmeChallengeKeyPattern); err == nil {
		for _, k := range keys {
			if b, err := sec.DecodeKey(k); err == nil {
				e.keyAuth[k] = b
			}
		}
	}
	t.tokens[p] = e
	return e.keyAuth
}
