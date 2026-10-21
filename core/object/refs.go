package object

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// RefsDocument is the keys of the sec and cfg an object references,
	// by datastore, named "<kind>/<name>" relative to the namespace of the
	// object, so a clone into another namespace lands its references
	// there. The values are decoded, base64 in json.
	RefsDocument map[string]map[string][]byte

	// RefsMode says how a restore applies a RefsDocument.
	RefsMode string
)

const (
	// RefsMissing creates the datastores and the keys absent, leaves the
	// keys there as they are, and reports the ones whose value differs.
	RefsMissing RefsMode = "missing"

	// RefsSkip applies no reference.
	RefsSkip RefsMode = "skip"

	// RefsOverwrite rewrites every key of the document.
	RefsOverwrite RefsMode = "overwrite"
)

// ParseRefsMode returns the mode s names, missing when empty.
func ParseRefsMode(s string) (RefsMode, error) {
	switch m := RefsMode(s); m {
	case "":
		return RefsMissing, nil
	case RefsMissing, RefsSkip, RefsOverwrite:
		return m, nil
	default:
		return "", fmt.Errorf("invalid refs mode %q: missing, skip or overwrite", s)
	}
}

// RefsDocumentOf returns the keys of the datastores at paths, decoded, named
// relative to the namespace ns.
func RefsDocumentOf(ns string, paths naming.Paths) (RefsDocument, error) {
	doc := make(RefsDocument)
	for _, p := range paths {
		if p.Namespace != ns {
			return nil, fmt.Errorf("%s: a datastore out of the namespace %s", p, ns)
		}
		o, err := NewDataStore(p, WithVolatile(true))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		names, err := o.AllKeys()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		keys := make(map[string][]byte, len(names))
		for _, name := range names {
			b, err := o.DecodeKey(name)
			if err != nil {
				return nil, fmt.Errorf("%s key %s: %w", p, name, err)
			}
			keys[name] = b
		}
		doc[refsName(p)] = keys
	}
	return doc, nil
}

func refsName(p naming.Path) string {
	return p.Kind.String() + "/" + p.Name
}

// ApplyRefs writes the keys of the document into the datastores of the
// namespace ns, as mode says, and returns what it did and what it left: a
// datastore created, a key added, rewritten, or left with a value differing
// from the document's.
func ApplyRefs(ns string, doc RefsDocument, mode RefsMode) ([]string, error) {
	if mode == RefsSkip {
		return nil, nil
	}
	names := make([]string, 0, len(doc))
	for name := range doc {
		names = append(names, name)
	}
	sort.Strings(names)
	notes := make([]string, 0)
	for _, name := range names {
		kind, objName, ok := strings.Cut(name, "/")
		if !ok || !slices.Contains([]string{naming.KindSec.String(), naming.KindCfg.String()}, kind) {
			return notes, fmt.Errorf("refs: %q is not a sec or cfg named <kind>/<name>", name)
		}
		p, err := naming.NewPath(ns, naming.ParseKind(kind), objName)
		if err != nil {
			return notes, fmt.Errorf("refs: %s: %w", name, err)
		}
		if !p.Exists() {
			notes = append(notes, fmt.Sprintf("create %s", p))
		}
		o, err := NewDataStore(p)
		if err != nil {
			return notes, fmt.Errorf("%s: %w", p, err)
		}
		keyNames := make([]string, 0, len(doc[name]))
		for k := range doc[name] {
			keyNames = append(keyNames, k)
		}
		sort.Strings(keyNames)
		for _, k := range keyNames {
			value := doc[name][k]
			if !o.HasKey(k) {
				if err := o.AddKey(k, value); err != nil {
					return notes, fmt.Errorf("%s key %s: %w", p, k, err)
				}
				notes = append(notes, fmt.Sprintf("add %s key %s", p, k))
				continue
			}
			current, err := o.DecodeKey(k)
			if err != nil {
				return notes, fmt.Errorf("%s key %s: %w", p, k, err)
			}
			if bytes.Equal(current, value) {
				continue
			}
			if mode == RefsOverwrite {
				if err := o.ChangeKey(k, value); err != nil {
					return notes, fmt.Errorf("%s key %s: %w", p, k, err)
				}
				notes = append(notes, fmt.Sprintf("rewrite %s key %s", p, k))
				continue
			}
			notes = append(notes, fmt.Sprintf("leave %s key %s, its value differs from the snapshot's", p, k))
		}
	}
	return notes, nil
}
