package driverdb_test

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/testhelper"

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
				if kw.Converter == xconfig.NodesConverter || kw.Converter == xconfig.PeersConverter {
					envTest := testhelper.Setup(t)
					envTest.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
					require.NoError(t, os.Unsetenv(env.ContextVar))
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

// TestKeywordAttrExistsOnDriver verifies the Attr of every manifest attribute
// of every registered driver names a field the driver struct really has.
//
// A keyword is bound to its field by name, resolved by reflection when the
// configuration is loaded, so a keyword naming a field no longer there, or
// never added, is only found when a configuration sets it, or even later when
// a driver shares its keywords with a driver of another group and only one of
// the two carries the field. The error then surfaced ("Specified field is not
// present in the struct") aborts every action on the resource, so this is
// checked here, once, for every driver.
func TestKeywordAttrExistsOnDriver(t *testing.T) {
	for _, drvID := range driver.List() {
		factory := resource.NewResourceFunc(drvID)
		if factory == nil {
			// node drivers have no factory, and thus no manifest
			continue
		}
		t.Run(drvID.String(), func(t *testing.T) {
			r := factory()
			for _, a := range manifest.Get(r).Attrs {
				name := a.Name()
				if name == "" {
					continue
				}
				assert.NoErrorf(t, hasAttr(r, name), "%s: attr %q", drvID, name)
			}
		})
	}
}

// hasAttr resolves a dotted manifest attribute path against the type of a
// resource, the way keywords.Keyword.SetValue resolves it against its value.
func hasAttr(r any, path string) error {
	t := reflect.TypeOf(r)
	for _, name := range strings.Split(path, ".") {
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return fmt.Errorf("%s is not a struct", t)
		}
		field, ok := t.FieldByName(name)
		if !ok {
			return fmt.Errorf("%s has no %s field", t, name)
		}
		if field.PkgPath != "" {
			return fmt.Errorf("%s.%s is unexported", t, name)
		}
		t = field.Type
	}
	return nil
}
