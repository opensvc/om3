// Package console is what the console sessions share between the node
// serving them and the clients attaching to them: the ticket a session is
// opened with, and the messages exchanged on its websocket.
package console

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// TokenUse is the token_use claim of a console ticket. It tells a
	// ticket from the access tokens signed with the same key: a ticket
	// is no credential of the api, and an access token opens no console.
	TokenUse = "console"

	// TicketDuration is how long a ticket is accepted for. It bounds the
	// time to connect, not the session: a session opened in time lasts
	// until its client or its command ends it.
	TicketDuration = 30 * time.Second

	// KindTTY is the kind of a console running a command in a terminal.
	KindTTY = "tty"
)

type (
	// Target is what a console session attaches to: a resource of an
	// instance, on the node running it.
	Target struct {
		// Node is the node the session is served on.
		Node string `json:"node"`

		// Path is the object path.
		Path string `json:"path"`

		// RID is the resource id, empty for the only resource of the
		// object a console can be opened on.
		RID string `json:"rid,omitempty"`

		// Kind is the kind of console.
		Kind string `json:"kind"`
	}

	// Ticket is what a client presents to open a console session. It is
	// signed by the node that checked the user may open it.
	Ticket struct {
		Target   Target `json:"console"`
		TokenUse string `json:"token_use"`
		jwt.RegisteredClaims
	}
)

// User is who the ticket was issued to.
func (t Ticket) User() string {
	return t.Subject
}

// NewTicket returns a ticket for the user to open a console on the target,
// issued by the node named.
func NewTicket(user, issuer string, target Target) (*Ticket, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	now := time.Now()
	return &Ticket{
		Target:   target,
		TokenUse: TokenUse,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user,
			Issuer:    issuer,
			ID:        hex.EncodeToString(b),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TicketDuration)),
		},
	}, nil
}

// Claims returns the claims of the ticket, less its expiration, for a signer
// that sets the expiration itself.
func (t *Ticket) Claims() map[string]any {
	return map[string]any{
		"sub":       t.Subject,
		"iss":       t.Issuer,
		"jti":       t.ID,
		"iat":       t.IssuedAt.Unix(),
		"token_use": t.TokenUse,
		"console":   t.Target,
	}
}

// Sign returns the ticket signed with the key the cluster signs its tokens
// with.
func (t *Ticket) Sign(key *rsa.PrivateKey) (string, error) {
	if key == nil {
		return "", errors.New("no key to sign the console ticket with")
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, t).SignedString(key)
}

// ParseTicket verifies a signed ticket and returns it. A token that is not a
// console ticket is refused, however valid its signature: the access tokens
// of the api are signed with the same key.
func ParseTicket(s string, key *rsa.PublicKey) (*Ticket, error) {
	t := &Ticket{}
	if _, err := jwt.ParseWithClaims(s, t,
		func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	); err != nil {
		return nil, fmt.Errorf("console ticket: %w", err)
	}
	switch {
	case t.TokenUse != TokenUse:
		return nil, errors.New("console ticket: not a console ticket")
	case t.ID == "":
		return nil, errors.New("console ticket: no id")
	case t.Subject == "":
		return nil, errors.New("console ticket: no user")
	case t.Target.Node == "" || t.Target.Path == "" || t.Target.Kind == "":
		return nil, errors.New("console ticket: incomplete target")
	}
	return t, nil
}
