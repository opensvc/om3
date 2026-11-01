// Package vsockauth is what proves a hypervisor to the daemon of its guest,
// over a vsock connection.
//
// A source port of the hypervisor below 1024 only says the process holds
// CAP_NET_BIND_SERVICE, not that it is the daemon: a service granted that
// capability alone would be root on the guests. So the daemon of the hypervisor
// signs a token, with the key of its cluster only root reads, and the guest
// verifies it with the public key it finds in its firmware, written there by
// the definition of the domain.
//
// The firmware is the qemu command line, which every user of the hypervisor
// reads: that is why it holds a public key, and not a secret. What only the
// hypervisor can do is writing it, and the guest reads it as root.
package vsockauth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// TokenUse is the token_use claim of a token of a hypervisor. The key
	// signing it also signs the access tokens of the api of its cluster,
	// which refuses this one.
	TokenUse = "vsock"

	// OEMPrefix starts the OEM string of the firmware of a guest holding the
	// public key of its hypervisor.
	OEMPrefix = "opensvc-hypervisor-key="

	// DMIEntries is where the kernel of a guest shows its firmware tables,
	// to root alone.
	DMIEntries = "/sys/firmware/dmi/entries"

	// ProductUUID is the file holding the uuid of the domain in a guest, to
	// root alone.
	ProductUUID = "/sys/class/dmi/id/product_uuid"

	// oemStringsType is the type of the OEM Strings table of the firmware.
	oemStringsType = 11

	// tokenDuration is how long a token is valid, for a client to make the
	// requests of an operation with it. It only ever goes to the guest it is
	// made for, the audience.
	tokenDuration = time.Hour

	// clockSkew is what the clock of a guest may be off by.
	clockSkew = 5 * time.Minute
)

// ErrNoKey says the firmware of the guest holds no key of a hypervisor: the
// domain has not started since it has one, or was not defined for it.
var ErrNoKey = errors.New("no hypervisor key in the firmware")

type claims struct {
	TokenUse string `json:"token_use"`
	jwt.RegisteredClaims
}

// LoadSignKey reads the key a hypervisor signs with.
func LoadSignKey(file string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPrivateKeyFromPEM(b)
}

// OEMString returns the OEM string telling a guest the key of its hypervisor.
func OEMString(key *rsa.PublicKey) (string, error) {
	b, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}
	return OEMPrefix + base64.StdEncoding.EncodeToString(b), nil
}

// NewToken returns a token proving the holder of the key to the guest of the
// uuid.
func NewToken(key *rsa.PrivateKey, guest string) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodRS256, &claims{
		TokenUse: TokenUse,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "hypervisor",
			Audience:  jwt.ClaimStrings{normalizeUUID(guest)},
			ID:        hex.EncodeToString(id),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenDuration)),
		},
	}).SignedString(key)
}

// Verify says s is a token of the holder of the key for the guest of the uuid.
func Verify(s string, key *rsa.PublicKey, guest string) error {
	c := &claims{}
	_, err := jwt.ParseWithClaims(s, c,
		func(*jwt.Token) (any, error) { return key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithAudience(normalizeUUID(guest)),
		jwt.WithLeeway(clockSkew),
	)
	if err != nil {
		return err
	}
	if c.TokenUse != TokenUse {
		return fmt.Errorf("token use %q is not %q", c.TokenUse, TokenUse)
	}
	return nil
}

// GuestKey returns the key of the hypervisor in the firmware tables of dir.
func GuestKey(dir string) (*rsa.PublicKey, error) {
	files, err := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%d-*", oemStringsType), "raw"))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		for _, s := range oemStrings(b) {
			if v, ok := strings.CutPrefix(s, OEMPrefix); ok {
				return parseKey(v)
			}
		}
	}
	return nil, ErrNoKey
}

// GuestUUID returns the uuid of the domain in the file, ProductUUID.
func GuestUUID(file string) (string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return normalizeUUID(string(b)), nil
}

func parseKey(s string) (*rsa.PublicKey, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("hypervisor key: %w", err)
	}
	i, err := x509.ParsePKIXPublicKey(b)
	if err != nil {
		return nil, fmt.Errorf("hypervisor key: %w", err)
	}
	key, ok := i.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("hypervisor key: not a rsa key")
	}
	return key, nil
}

// oemStrings returns the strings of a raw OEM Strings table: a header of the
// length its second byte says, then the strings, each ended by a zero.
func oemStrings(b []byte) []string {
	if len(b) < 2 || b[0] != oemStringsType || int(b[1]) > len(b) {
		return nil
	}
	l := make([]string, 0)
	for _, s := range bytes.Split(b[b[1]:], []byte{0}) {
		if len(s) > 0 {
			l = append(l, string(s))
		}
	}
	return l
}

func normalizeUUID(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
