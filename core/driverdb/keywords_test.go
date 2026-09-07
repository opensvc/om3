package driverdb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"

	// Register all the resource drivers, so their manifest keywords are
	// reachable from driver.List() and object.KeywordStoreWithDrivers().
	_ "github.com/opensvc/om3/v3/core/driverdb"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/converters"
)

// TestKeywordConverterIsRegistered verifies every keyword of the node, of every
// object kind and of every registered driver uses a converter registered in the
// converters DB.
//
// The keyword Converter is a converters.Converter, so an unknown converter can
// no longer be named by mistake. This test is the remaining guard, for a
// converter implemented but not registered.
func TestKeywordConverterIsRegistered(t *testing.T) {
	for name, store := range allKeywordStores(t) {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, store)
			for _, kw := range store {
				if kw.Converter == nil {
					continue
				}
				name := kw.Converter.String()
				registered, ok := converters.Get(name)
				require.Truef(t, ok, "%s: converter %q is not registered", kwID(kw), name)
				assert.Equalf(t, registered, kw.Converter,
					"%s: converter %q is registered as a different converter", kwID(kw), name)
			}
		})
	}
}

// TestKeywordDefaultIsConvertible verifies the default value of every keyword is
// accepted by the keyword converter.
//
// Defaults holding a reference are skipped: they are only convertible once
// dereferenced against a real configuration.
func TestKeywordDefaultIsConvertible(t *testing.T) {
	for name, store := range allKeywordStores(t) {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, store)
			for _, kw := range store {
				if kw.Converter == nil || kw.Default == "" {
					continue
				}
				if hasReference(kw.Default) {
					continue
				}
				_, err := kw.Converter.Convert(kw.Default)
				assert.NoErrorf(t, err, "%s: default %q is not convertible by the %s converter",
					kwID(kw), kw.Default, kw.Converter)
			}
		})
	}
}

// allKeywordStores returns the node keyword store, the keyword store of every
// object kind, and the keyword store of every registered driver.
//
// The per-driver stores are needed because a driver manifest declaring no kind
// is not reachable from object.KeywordStoreWithDrivers().
func allKeywordStores(t *testing.T) map[string]keywords.Store {
	t.Helper()
	m := map[string]keywords.Store{
		"node": object.NodeKeywordStore,
	}
	for _, kind := range naming.KindAll {
		m[kind.String()] = object.KeywordStoreWithDrivers(kind)
	}
	for _, drvID := range driver.List() {
		factory := resource.NewResourceFunc(drvID)
		if factory == nil {
			// node drivers have no factory, and thus no manifest
			continue
		}
		store := keywords.Store(manifest.Get(factory()).Keywords())
		if len(store) == 0 {
			continue
		}
		m["driver "+drvID.String()] = store
	}
	return m
}

func kwID(kw *keywords.Keyword) string {
	s := kw.Section + "." + kw.Option
	if len(kw.Types) > 0 {
		s += " (type " + kw.Types[0] + ")"
	}
	return s
}

func hasReference(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			return true
		}
	}
	return false
}
