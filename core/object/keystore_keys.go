package object

import (
	"path/filepath"
	"sort"

	"github.com/danwakefield/fnmatch"
	"opensvc.com/opensvc/util/xmap"
)

// OptsKeys is the options of the Keys function of all data objects.
type OptsKeys struct {
	OptsGlobal
	OptsLock
	Match string `flag:"match"`
}

// Get returns a keyword value
func (t *Keystore) Keys(options OptsKeys) ([]string, error) {
	return t.MatchingKeys(options.Match)
}

//
// MatchingDirs returns the list of all directories and parent directories
// hosting keys in the store's virtual filesystem.
//
// Example: []key{"a/b/c", "a/c/b"} => []dir{"a", "a/b", "a/c"}
//
func (t *Keystore) MatchingDirs(pattern string) ([]string, error) {
	m := make(map[string]interface{})
	keys, err := t.MatchingKeys(pattern)
	if err != nil {
		return []string{}, err
	}
	for _, k := range keys {
		for {
			k = filepath.Dir(k)
			if k == "" || k == "/" || k == "." {
				break
			}
			m[k] = nil
		}
	}
	dirs := xmap.Keys(m)
	sort.Strings(dirs)
	return dirs, nil
}

func (t *Keystore) AllDirs() ([]string, error) {
	return t.MatchingDirs("")
}

func (t *Keystore) AllKeys() ([]string, error) {
	return t.MatchingKeys("")
}

func (t *Keystore) MatchingKeys(pattern string) ([]string, error) {
	data := make([]string, 0)
	f := fnmatch.FNM_PATHNAME | fnmatch.FNM_LEADING_DIR

	for _, s := range t.config.Keys(DataSectionName) {
		if pattern == "" || fnmatch.Match(pattern, s, f) {
			data = append(data, s)
		}
	}
	return data, nil
}
