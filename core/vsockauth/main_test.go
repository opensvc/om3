package vsockauth

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const guest = "de857c8e-280f-49ba-a9e5-b488688f2c8e"

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func sign(t *testing.T, key *rsa.PrivateKey, c *claims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(key)
	require.NoError(t, err)
	return s
}

func TestToken(t *testing.T) {
	key := newKey(t)
	s, err := NewToken(key, guest)
	require.NoError(t, err)
	assert.NoError(t, Verify(s, &key.PublicKey, guest))
	assert.NoError(t, Verify(s, &key.PublicKey, "DE857C8E-280F-49BA-A9E5-B488688F2C8E\n"), "as a guest may tell it")

	assert.Error(t, Verify(s, &newKey(t).PublicKey, guest), "the key of another hypervisor")
	assert.Error(t, Verify(s, &key.PublicKey, "20b1a61c-605b-4a9d-9a57-a75f5e0d3db0"), "made for another guest")

	valid := jwt.RegisteredClaims{
		Audience:  jwt.ClaimStrings{guest},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	assert.NoError(t, Verify(sign(t, key, &claims{TokenUse: TokenUse, RegisteredClaims: valid}), &key.PublicKey, guest))
	assert.Error(t, Verify(sign(t, key, &claims{TokenUse: "access", RegisteredClaims: valid}), &key.PublicKey, guest), "an access token of the cluster")

	expired := valid
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))
	assert.Error(t, Verify(sign(t, key, &claims{TokenUse: TokenUse, RegisteredClaims: expired}), &key.PublicKey, guest))

	never := valid
	never.ExpiresAt = nil
	assert.Error(t, Verify(sign(t, key, &claims{TokenUse: TokenUse, RegisteredClaims: never}), &key.PublicKey, guest), "no expiration")

	skewed := valid
	skewed.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
	assert.NoError(t, Verify(sign(t, key, &claims{TokenUse: TokenUse, RegisteredClaims: skewed}), &key.PublicKey, guest), "the clock of a guest may be ahead")

	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, &claims{TokenUse: TokenUse, RegisteredClaims: valid}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	assert.Error(t, Verify(none, &key.PublicKey, guest), "no signature")
}

// A table as the kernel shows it: type, length, handle, count, then the strings.
func table(strings ...string) []byte {
	b := []byte{oemStringsType, 5, 0, 1, byte(len(strings))}
	for _, s := range strings {
		b = append(b, s...)
		b = append(b, 0)
	}
	return append(b, 0)
}

func writeTable(t *testing.T, dir, name string, b []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, name), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name, "raw"), b, 0o400))
}

func TestGuestKey(t *testing.T) {
	key := newKey(t)
	oem, err := OEMString(&key.PublicKey)
	require.NoError(t, err)

	dir := t.TempDir()
	_, err = GuestKey(dir)
	assert.ErrorIs(t, err, ErrNoKey, "no table")

	writeTable(t, dir, "11-0", table("vboxVer_7.0", "other"))
	writeTable(t, dir, "1-0", table(oem))
	_, err = GuestKey(dir)
	assert.ErrorIs(t, err, ErrNoKey, "no such string, and a table of another type")

	writeTable(t, dir, "11-1", table("first", oem))
	got, err := GuestKey(dir)
	require.NoError(t, err)
	assert.True(t, key.PublicKey.Equal(got))

	writeTable(t, dir, "11-1", table(OEMPrefix+"!!"))
	_, err = GuestKey(dir)
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoKey)
}

func TestOEMStringsOfATruncatedTable(t *testing.T) {
	assert.Empty(t, oemStrings(nil))
	assert.Empty(t, oemStrings([]byte{oemStringsType}))
	assert.Empty(t, oemStrings([]byte{oemStringsType, 200, 0}))
	assert.Empty(t, oemStrings([]byte{1, 5, 0, 0, 1, 'a', 0, 0}))
}

func TestGuestUUID(t *testing.T) {
	file := filepath.Join(t.TempDir(), "product_uuid")
	require.NoError(t, os.WriteFile(file, []byte("DE857C8E-280F-49BA-A9E5-B488688F2C8E\n"), 0o400))
	s, err := GuestUUID(file)
	require.NoError(t, err)
	assert.Equal(t, guest, s)
}

func TestLoadSignKey(t *testing.T) {
	_, err := LoadSignKey(filepath.Join(t.TempDir(), "none"))
	assert.Error(t, err)
}
