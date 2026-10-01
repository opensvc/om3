package console

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

var testTarget = Target{Node: "n1", Path: "ns1/svc/web", RID: "container#1", Kind: KindTTY}

// A ticket signed by the cluster key is read back with what it was issued
// for, and each ticket has an id of its own.
func TestTicketRoundTrip(t *testing.T) {
	key := testKey(t)
	ticket, err := NewTicket("alice", "n2", testTarget)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := ticket.Sign(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseTicket(signed, &key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if got.User() != "alice" || got.Issuer != "n2" || got.Target != testTarget || got.ID != ticket.ID {
		t.Errorf("got %+v", got)
	}
	other, err := NewTicket("alice", "n2", testTarget)
	if err != nil {
		t.Fatal(err)
	}
	if other.ID == ticket.ID {
		t.Error("two tickets have the same id")
	}
}

// The claims handed to a signer that sets the expiration itself make the
// same ticket.
func TestTicketClaims(t *testing.T) {
	key := testKey(t)
	ticket, err := NewTicket("alice", "n2", testTarget)
	if err != nil {
		t.Fatal(err)
	}
	claims := jwt.MapClaims{"exp": time.Now().Add(TicketDuration).Unix()}
	for k, v := range ticket.Claims() {
		claims[k] = v
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseTicket(signed, &key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if got.User() != "alice" || got.Target != testTarget || got.ID != ticket.ID {
		t.Errorf("got %+v", got)
	}
}

// A ticket is refused when it is signed by another key, expired, not a
// console ticket, as an access token of the api signed with the same key is,
// or incomplete.
func TestTicketRefused(t *testing.T) {
	key := testKey(t)
	sign := func(edit func(*Ticket), with *rsa.PrivateKey) string {
		ticket, err := NewTicket("alice", "n2", testTarget)
		if err != nil {
			t.Fatal(err)
		}
		edit(ticket)
		signed, err := ticket.Sign(with)
		if err != nil {
			t.Fatal(err)
		}
		return signed
	}
	for name, signed := range map[string]string{
		"another key":   sign(func(*Ticket) {}, testKey(t)),
		"expired":       sign(func(tk *Ticket) { tk.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }, key),
		"no expiration": sign(func(tk *Ticket) { tk.ExpiresAt = nil }, key),
		"access token":  sign(func(tk *Ticket) { tk.TokenUse = "access" }, key),
		"no token use":  sign(func(tk *Ticket) { tk.TokenUse = "" }, key),
		"no id":         sign(func(tk *Ticket) { tk.ID = "" }, key),
		"no user":       sign(func(tk *Ticket) { tk.Subject = "" }, key),
		"no node":       sign(func(tk *Ticket) { tk.Target.Node = "" }, key),
		"no path":       sign(func(tk *Ticket) { tk.Target.Path = "" }, key),
		"garbage":       "not.a.ticket",
	} {
		if _, err := ParseTicket(signed, &key.PublicKey); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
