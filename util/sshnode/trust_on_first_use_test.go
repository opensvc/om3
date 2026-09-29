package sshnode

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"
)

func newTestHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// The key of a host seen for the first time is trusted and recorded, the
// same key is trusted again, and another key of that host is refused.
func TestTrustOnFirstUseHostKeyCallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	check, err := TrustOnFirstUseHostKeyCallback()
	if err != nil {
		t.Fatal(err)
	}
	remote := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}
	first, other := newTestHostKey(t), newTestHostKey(t)

	if err := check("sansw1:22", remote, first); err != nil {
		t.Fatalf("first contact refused: %s", err)
	}
	if err := check("sansw1:22", remote, first); err != nil {
		t.Fatalf("known key refused: %s", err)
	}
	if err := check("sansw1:22", remote, other); err == nil {
		t.Fatal("changed key accepted")
	}
	if err := check("sansw2:22", remote, other); err != nil {
		t.Fatalf("first contact of another host refused: %s", err)
	}

	// A host on another port is recorded under that port, and known on
	// the next connection.
	if err := check("sansw3:2222", remote, first); err != nil {
		t.Fatalf("first contact on another port refused: %s", err)
	}
	if err := check("sansw3:2222", remote, other); err == nil {
		t.Fatal("changed key on another port accepted")
	}
}
