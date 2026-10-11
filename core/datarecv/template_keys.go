package datarecv

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	// templateKey is a datastore key a template read.
	templateKey struct {
		Path string `json:"path"`
		Key  string `json:"key"`
	}

	// templateKeys are the keys each template of a resource read at its
	// last render, by template: "<datastore path>:<key>".
	//
	// A change of one of them installs the files of the template again,
	// as a change of the template key itself does, so a file rendered
	// from a sec follows the sec.
	templateKeys map[string][]templateKey
)

func templateID(store naming.Path, k string) string {
	return store.String() + ":" + k
}

// templateKeysFile returns the file recording the keys the templates of the
// resource read, none when the receiver is not a resource of an object.
func (t *DataRecv) templateKeysFile() string {
	o, ok := t.to.GetObject().(object.Core)
	if !ok {
		return ""
	}
	r, ok := t.to.(ridder)
	if !ok {
		return ""
	}
	return filepath.Join(o.Path().VarDir(), r.RID(), "template_keys")
}

func readTemplateKeys(filename string) templateKeys {
	m := make(templateKeys)
	if filename == "" {
		return m
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

// recordTemplateKeys records keys as the ones the template id read, when
// they differ from the ones recorded.
func (t *DataRecv) recordTemplateKeys(id string, keys []templateKey) {
	filename := t.templateKeysFile()
	if filename == "" {
		return
	}
	m := readTemplateKeys(filename)
	if slices.Equal(m[id], keys) {
		return
	}
	if len(keys) == 0 {
		delete(m, id)
	} else {
		m[id] = keys
	}
	if err := writeTemplateKeys(filename, m); err != nil {
		t.to.Log().Warnf("record the keys template %s read: %s", id, err)
	}
}

func writeTemplateKeys(filename string, m templateKeys) error {
	if len(m) == 0 {
		if err := os.Remove(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filename)
}

// reads says if a template of the datastore store read the key k of the
// datastore path, any of its keys when k is empty.
func (m templateKeys) reads(store, path naming.Path, k string) bool {
	prefix := store.String() + ":"
	for id, keys := range m {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		for _, e := range keys {
			if e.Path == path.String() && (k == "" || e.Key == k) {
				return true
			}
		}
	}
	return false
}

// readsFrom says if a template of a resource read the key k of the datastore
// path, any of its keys when k is empty.
func (m templateKeys) readsFrom(path naming.Path, k string) bool {
	for _, keys := range m {
		for _, e := range keys {
			if e.Path == path.String() && (k == "" || e.Key == k) {
				return true
			}
		}
	}
	return false
}
