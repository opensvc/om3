package getaddr

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLookupIP simule net.LookupIP pour les tests.
var mockLookupIP = func(name string) ([]net.IP, error) {
	return nil, errors.New("not implemented")
}

// saveOriginalLookupIP sauvegarde la fonction originale netLookupIP.
var originalLookupIP func(string) ([]net.IP, error)

// TestMain configure les tests et nettoie après.
func TestMain(m *testing.M) {
	// Sauvegarder la fonction originale netLookupIP
	originalLookupIP = netLookupIP
	// Remplacer par notre mock
	netLookupIP = mockLookupIP

	// Exécuter les tests
	code := m.Run()

	// Restaurer la fonction originale
	netLookupIP = originalLookupIP

	os.Exit(code)
}

func TestErrManyAddr_Error(t *testing.T) {
	err := ErrManyAddr{name: "example.com", count: 3}
	assert.Equal(t, "name example.com resolves to 3 address", err.Error())
}

func TestErrCacheAddr_Error(t *testing.T) {
	err := ErrCacheAddr{name: "example.com"}
	assert.Equal(t, "error caching the name example.com addr", err.Error())
}

func TestFmtCacheFile(t *testing.T) {
	// Sauvegarder la valeur originale de cacheDir
	originalCacheDir := cacheDir
	// Définir un chemin temporaire pour les tests
	cacheDir = filepath.Join(os.TempDir(), "test_cache_dir")

	// Tester fmtCacheFile
	filename := fmtCacheFile("example.com")
	expected := filepath.Join(cacheDir, "example.com")
	assert.Equal(t, expected, filename)

	// Restaurer cacheDir
	cacheDir = originalCacheDir
}

func TestLoad(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tempDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tempDir

	// Créer un fichier de cache factice
	cacheFile := filepath.Join(tempDir, "example.com")
	ip := net.ParseIP("192.168.1.1")
	err := os.WriteFile(cacheFile, []byte(ip.String()), 0o644)
	require.NoError(t, err)

	// Tester load
	loadedIP, duration, err := load("example.com")
	require.NoError(t, err)
	assert.Equal(t, ip, loadedIP)
	assert.True(t, duration > 0)

	// Tester avec un fichier inexistant
	_, _, err = load("nonexistent.com")
	assert.Error(t, err)

	// Restaurer cacheDir
	cacheDir = originalCacheDir
}

func TestCache(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tempDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tempDir

	// Tester cache avec un IP valide
	ip := net.ParseIP("192.168.1.1")
	err := cache("example.com", ip)
	require.NoError(t, err)

	// Vérifier que le fichier a été créé
	cacheFile := filepath.Join(tempDir, "example.com")
	_, err = os.Stat(cacheFile)
	assert.NoError(t, err)

	// Tester avec un IP invalide (nil)
	err = cache("example.com", nil)
	assert.Error(t, err)

	// Restaurer cacheDir
	cacheDir = originalCacheDir
}

func TestLookup(t *testing.T) {
	tests := []struct {
		name     string
		mockFunc func(string) ([]net.IP, error)
		expected net.IP
		err      error
	}{
		{
			name: "single IP",
			mockFunc: func(name string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("192.168.1.1")}, nil
			},
			expected: net.ParseIP("192.168.1.1"),
			err:      nil,
		},
		{
			name: "no IP",
			mockFunc: func(name string) ([]net.IP, error) {
				return []net.IP{}, nil
			},
			expected: nil,
			err:      errors.New("name example.com is unresolvable"),
		},
		{
			name: "multiple IPs",
			mockFunc: func(name string) ([]net.IP, error) {
				return []net.IP{
					net.ParseIP("192.168.1.1"),
					net.ParseIP("192.168.1.2"),
				}, nil
			},
			expected: net.ParseIP("192.168.1.1"),
			err:      ErrManyAddr{name: "example.com", count: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Remplacer le mock
			netLookupIP = tt.mockFunc

			ip, err := lookup("example.com")
			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, ip)
		})
	}

	// Restaurer le mock original
	netLookupIP = mockLookupIP
}

func TestLookupAndCache(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tempDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tempDir

	tests := []struct {
		name     string
		input    string
		mockFunc func(string) ([]net.IP, error)
		expected net.IP
		err      error
	}{
		{
			name:     "valid IP string",
			input:    "192.168.1.1",
			expected: net.ParseIP("192.168.1.1"),
			err:      nil,
		},
		{
			name:     "invalid IP string",
			input:    "1.2.3.?",
			expected: nil,
			err:      errors.New("unparsable ip 1.2.3.?"),
		},
		{
			name:  "valid FQDN",
			input: "example.com",
			mockFunc: func(name string) ([]net.IP, error) {
				return []net.IP{net.ParseIP("192.168.1.1")}, nil
			},
			expected: net.ParseIP("192.168.1.1"),
			err:      nil,
		},
		{
			name:  "unresolvable FQDN",
			input: "unresolvable.com",
			mockFunc: func(name string) ([]net.IP, error) {
				return []net.IP{}, nil
			},
			expected: nil,
			err:      errors.New("name unresolvable.com is unresolvable"),
		},
		{
			name:  "FQDN with multiple IPs",
			input: "multihomed.com",
			mockFunc: func(name string) ([]net.IP, error) {
				return []net.IP{
					net.ParseIP("192.168.1.1"),
					net.ParseIP("192.168.1.2"),
				}, nil
			},
			expected: net.ParseIP("192.168.1.1"),
			err:      ErrManyAddr{name: "multihomed.com", count: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Remplacer le mock
			netLookupIP = tt.mockFunc

			ip, err := lookupAndCache(tt.input)
			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, ip)

			// Vérifier que le cache a été créé si nécessaire
			if tt.err == nil && naming.IsValidFQDN(tt.input) {
				cacheFile := filepath.Join(tempDir, tt.input)
				_, err := os.Stat(cacheFile)
				assert.NoError(t, err)
			}
		})
	}

	// Restaurer le mock original et cacheDir
	netLookupIP = mockLookupIP
	cacheDir = originalCacheDir
}

func TestLookupFunction(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tempDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tempDir

	// Créer un fichier de cache factice
	cacheFile := filepath.Join(tempDir, "example.com")
	ip := net.ParseIP("192.168.1.1")
	err := os.WriteFile(cacheFile, []byte(ip.String()), 0o644)
	require.NoError(t, err)

	// Définir un mock pour netLookupIP qui échoue
	netLookupIP = func(name string) ([]net.IP, error) {
		return nil, errors.New("lookup failed")
	}

	// Tester Lookup avec un cache existant
	loadedIP, duration, err := Lookup("example.com")
	require.NoError(t, err)
	assert.Equal(t, ip, loadedIP)
	assert.True(t, duration > 0)

	// Tester Lookup sans cache
	_, _, err = Lookup("nonexistent.com")
	assert.Error(t, err)

	// Restaurer cacheDir et le mock original
	cacheDir = originalCacheDir
	netLookupIP = originalLookupIP
}
