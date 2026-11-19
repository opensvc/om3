package omcmd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/omcrypto"
	"github.com/opensvc/om3/v3/util/hostname"
)

func writeSecConf(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "cert.conf")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
	return file
}

// The speaker issues the certificate for the cluster nodes when its
// alt_names holds {clusternodes}, as by default, and it is not of an acme
// directory: only then does a joining node wait for it to name the node.
func TestCertIssuedForClusterNodes(t *testing.T) {
	assert.True(t, certIssuedForClusterNodes(writeSecConf(t, "[DEFAULT]\nvalidity = 1y\n")), "alt_names by default")
	assert.True(t, certIssuedForClusterNodes(writeSecConf(t, "[DEFAULT]\nalt_names = {clusternodes} vip.example.com\n")))
	assert.False(t, certIssuedForClusterNodes(writeSecConf(t, "[DEFAULT]\nalt_names = n1 n2\n")), "a fixed list")
	assert.False(t, certIssuedForClusterNodes(writeSecConf(t, "[DEFAULT]\n\n[acme]\ndirectory = letsencrypt\n")), "an acme directory")
	assert.True(t, certIssuedForClusterNodes(writeSecConf(t, "[DEFAULT]\n\n[acme]\nrenew_before = 30d\n")), "an acme section without a directory")
	assert.False(t, certIssuedForClusterNodes(filepath.Join(t.TempDir(), "nosuch")))
}

// stubCertNames says the certificate of the files of names names the node.
func stubCertNames(names map[string]bool) func(string) (bool, error) {
	return func(file string) (bool, error) { return names[file], nil }
}

// A joining node fetches the certificate again until it names the node, the
// speaker reissuing it for the cluster nodes the join added the node to.
func TestWaitCertNamingNode(t *testing.T) {
	old := writeSecConf(t, "[DEFAULT]\n")
	renewed := writeSecConf(t, "[DEFAULT]\n")
	certNames := stubCertNames(map[string]bool{renewed: true})
	fetches := 0
	fetch := func() (string, time.Time, error) {
		fetches++
		if fetches < 3 {
			return old, time.Time{}, nil
		}
		return renewed, time.Unix(3, 0), nil
	}
	file, ts := (&CmdClusterJoin{}).waitCertNamingNode(context.Background(), old, time.Time{}, fetch, certNames, time.Millisecond, time.Second)
	assert.Equal(t, renewed, file)
	assert.Equal(t, time.Unix(3, 0), ts)
	assert.Equal(t, 3, fetches)
}

// The wait is bounded: the join goes on with the certificate fetched.
func TestWaitCertNamingNodeTimeout(t *testing.T) {
	old := writeSecConf(t, "[DEFAULT]\n")
	certNames := stubCertNames(map[string]bool{})
	fetch := func() (string, time.Time, error) { return old, time.Time{}, nil }
	begin := time.Now()
	file, _ := (&CmdClusterJoin{}).waitCertNamingNode(context.Background(), old, time.Time{}, fetch, certNames, time.Millisecond, 50*time.Millisecond)
	assert.Equal(t, old, file)
	assert.Less(t, time.Since(begin), time.Second)
}

// A certificate already naming the node, or one the cluster does not issue
// for its nodes, is not waited for.
func TestWaitCertNamingNodeNotWaited(t *testing.T) {
	named := writeSecConf(t, "[DEFAULT]\n")
	fixed := writeSecConf(t, "[DEFAULT]\nalt_names = n1 n2\n")
	certNames := stubCertNames(map[string]bool{named: true})
	fetch := func() (string, time.Time, error) {
		t.Fatal("no fetch expected")
		return "", time.Time{}, nil
	}
	file, _ := (&CmdClusterJoin{}).waitCertNamingNode(context.Background(), named, time.Time{}, fetch, certNames, time.Millisecond, time.Second)
	assert.Equal(t, named, file)
	file, _ = (&CmdClusterJoin{}).waitCertNamingNode(context.Background(), fixed, time.Time{}, fetch, certNames, time.Millisecond, time.Second)
	assert.Equal(t, fixed, file)
}

// The certificate fetched from the cluster joined is decrypted with the
// secret of that cluster, read from its configuration fetched with it, not
// with the one of the cluster the node was in: every join to another
// cluster waited the whole timeout, the check failing.
func TestCertFileNamesWithTheSecretOfTheClusterJoined(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		DNSNames:     []string{"n1", "n2"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	chain := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	clusterConf := []byte("[cluster]\nname = joined\nsecret = 0123456789abcdef0123456789abcdef\n\n[pool#tank]\nname = tank\n")
	name, secret := clusterNameAndSecret(clusterConf)
	assert.Equal(t, "joined", name, "the name of the cluster section only")
	enc, err := (&omcrypto.T{NodeName: hostname.Hostname(), ClusterName: name, Key: secret}).Encrypt(chain)
	require.NoError(t, err)
	file := writeSecConf(t, "[DEFAULT]\n\n[data]\ncertificate_chain = crypt:"+base64.URLEncoding.Strict().EncodeToString(enc)+"\n")

	named, err := certFileNames(file, "n2", name, secret)
	require.NoError(t, err)
	assert.True(t, named)
	named, err = certFileNames(file, "n3", name, secret)
	require.NoError(t, err)
	assert.False(t, named)
	_, err = certFileNames(file, "n2", name, "another secret, another cluster")
	assert.Error(t, err)
	_, err = certFileNames(file, "n2", name, "")
	assert.Error(t, err, "no secret: an error, no panic")
	_, err = certFileNames(file, "n2", "", secret)
	assert.Error(t, err, "no name: an error, no panic")
}

// The cluster name and secret are read as the configurations are: ":" as
// "=", and inline comments.
func TestClusterNameAndSecretSyntax(t *testing.T) {
	for _, conf := range []string{
		"[cluster]\nname = c1\nsecret = s3cr3t\n",
		"[cluster]\nname: c1\nsecret: s3cr3t\n",
		"[cluster]\nname = c1 ; the name\nsecret = s3cr3t # the secret\n",
	} {
		name, secret := clusterNameAndSecret([]byte(conf))
		assert.Equal(t, "c1", name, conf)
		assert.Equal(t, "s3cr3t", secret, conf)
	}
	assert.True(t, certIssuedForClusterNodes(writeSecConf(t, "[DEFAULT]\nalt_names: {clusternodes}\n")))
}
