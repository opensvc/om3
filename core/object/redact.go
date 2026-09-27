package object

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/ini"
	"github.com/opensvc/om3/v3/util/key"
)

// redacted is what the value of a secret holding keyword is replaced with.
const redacted = "********"

// RedactedValue is what a secret value is shown as to a reader not allowed to
// see it.
const RedactedValue = redacted

// redactOptions reads a configuration file the way xconfig reads it, so the
// keys seen here are the keys the agent sees.
var redactOptions = ini.Options{
	Loose:                      true,
	AllowPythonMultilineValues: true,
	SpaceBeforeInlineComment:   true,
}

// RedactSecrets returns b with the value of every keyword declared with
// RedactSecret, and of every key of a sec or usr data section, replaced by
// asterisks. kind is the object kind the configuration belongs to, empty for a
// node or cluster configuration.
//
// Only those values change. The parser writes back verbatim the lines it was
// not asked to touch, so comments, blank lines, spacing, inline comments and
// multiline layout are all preserved, and the substitution is refused if it
// would not parse back to the redacted value.
//
// An unparseable configuration is an error rather than a configuration
// returned unredacted.
func RedactSecrets(b []byte, kind string) ([]byte, error) {
	f, err := ini.Load(redactOptions, b)
	if err != nil {
		return nil, fmt.Errorf("redact secrets: %w", err)
	}

	k := naming.Kind(kind)
	var store keywords.Store
	if k == naming.KindInvalid {
		store = NodeKeywordStore
	} else {
		store = KeywordStoreWithDrivers(k)
	}

	for _, section := range f.Sections() {
		sectionType := section.KeysHash()["type"]
		for _, sectionKey := range section.Keys() {
			if !isSecretKey(store, k, key.New(section.Name(), sectionKey.Name()), sectionType) {
				continue
			}
			sectionKey.SetValueKeepFormat(redacted)
		}
	}

	out, err := f.Bytes()
	if err != nil {
		return nil, fmt.Errorf("redact secrets: %w", err)
	}
	return out, nil
}

// IsSecretKey says whether the value of k, in a configuration of an object of
// kind, is a secret: a key of a sec or usr data section, or a keyword declared
// with RedactSecret. kind is invalid for a node or cluster configuration, and
// sectionType is the type of the section k is in.
func IsSecretKey(kind naming.Kind, k key.T, sectionType string) bool {
	var store keywords.Store
	if kind == naming.KindInvalid {
		store = NodeKeywordStore
	} else {
		store = KeywordStoreWithDrivers(kind)
	}
	return isSecretKey(store, kind, k, sectionType)
}

func isSecretKey(store keywords.Store, kind naming.Kind, k key.T, sectionType string) bool {
	// A datastore holds its keys as the values of the data section, so
	// there is no keyword to declare and all of them are secret.
	if (kind == naming.KindSec || kind == naming.KindUsr) && k.Section == "data" {
		return true
	}
	kw := store.Lookup(key.New(k.Section, k.BaseOption()), kind, sectionType)
	return kw != nil && kw.RedactSecret
}
