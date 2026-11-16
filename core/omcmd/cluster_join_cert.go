package omcmd

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	// certReissueTimeout bounds the wait of a joining node for the
	// certificate of the cluster to name it.
	certReissueTimeout = 15 * time.Second

	// certReissueInterval is the interval between the fetches of the
	// certificate of the cluster while it does not name the joining node.
	certReissueInterval = 300 * time.Millisecond
)

// waitCertNamingNode returns the certificate of the cluster fetched, once
// it names this node: fetched as the join adds the node to the cluster
// nodes, it is the one before the speaker reissues it for them, and the
// peers refuse the certificate the node presents until the node fetches
// the one reissued.
//
// A certificate the cluster does not issue for its nodes, as one of an
// alt_names not holding {clusternodes} or of an acme directory, is not
// waited for. The wait is bounded: the join goes on with the certificate
// fetched, warning, and the daemon fetches the one reissued later.
func (t *CmdClusterJoin) waitCertNamingNode(ctx context.Context, file string, timestamp time.Time, fetch func() (string, time.Time, error), certNames func(file string) (bool, error), interval, timeout time.Duration) (string, time.Time) {
	localhost := hostname.Hostname()
	// A certificate not read is not known to name the node: waited for,
	// as one not naming it.
	if named, _ := certNames(file); named {
		return file, timestamp
	}
	if !certIssuedForClusterNodes(file) {
		return file, timestamp
	}
	t.logger().Infof("Wait for %s to name %s", naming.SecCert, localhost)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.logger().Warnf("%s does not name %s after %s: the peers refuse its certificate until it fetches the one the speaker reissues", naming.SecCert, localhost, timeout)
			return file, timestamp
		case <-ticker.C:
		}
		f, ts, err := fetch()
		if err != nil {
			continue
		}
		file, timestamp = f, ts
		if named, _ := certNames(file); named {
			return file, timestamp
		}
	}
}

// certFileNames says whether the certificate of the sec configuration file
// names the node, its keys decrypted with the secret of the cluster named
// clusterName: the cluster joined, whose configuration is not installed yet.
func certFileNames(file, nodename, clusterName, secret string) (bool, error) {
	if clusterName == "" || secret == "" {
		// The decrypter refuses, panicking, an empty name or secret.
		return false, fmt.Errorf("no cluster name or secret to decrypt the certificate with")
	}
	value, err := iniValue(file, "data", "certificate_chain")
	if err != nil {
		return false, err
	}
	b, err := object.DecodeSecValueWith(value, clusterName, secret)
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return false, fmt.Errorf("certificate_chain holds no certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}
	return cert.VerifyHostname(nodename) == nil, nil
}

// clusterNameAndSecret returns the name and the secret of the cluster of
// the cluster configuration b, read as the configurations are, "=" or ":"
// separated, with inline comments.
func clusterNameAndSecret(b []byte) (string, string) {
	cfg, err := xconfig.NewObject("", b)
	if err != nil {
		return "", ""
	}
	return cfg.Get(key.New("cluster", "name")), cfg.Get(key.New("cluster", "secret"))
}

// iniValue returns the value of the option of the section of the
// configuration file.
func iniValue(file, section, option string) (string, error) {
	cfg, err := xconfig.NewObject("", file)
	if err != nil {
		return "", err
	}
	if v := cfg.Get(key.New(section, option)); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s: no %s.%s", file, section, option)
}

// certIssuedForClusterNodes says whether the speaker issues the certificate
// of the sec configuration file for the cluster nodes: its alt_names, as
// written, holds {clusternodes}, as by default, and it is not of an acme
// directory.
func certIssuedForClusterNodes(file string) bool {
	// The loader takes a missing file for an empty one.
	if _, err := os.Stat(file); err != nil {
		return false
	}
	cfg, err := xconfig.NewObject("", file)
	if err != nil {
		return false
	}
	// Of an acme directory: a section without one is no acme.
	if cfg.Get(key.New("acme", "directory")) != "" {
		return false
	}
	k := key.New("DEFAULT", "alt_names")
	return !cfg.HasKey(k) || strings.Contains(cfg.Get(k), "{clusternodes}")
}
