package object

import (
	"path/filepath"
	"sort"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/util/xmap"
)

func pathChain(k string) []string {
	m := make(map[string]any)
	for {
		k = filepath.Dir(k)
		if k == "" || k == "/" || k == "." {
			break
		}
		m[k] = nil
	}
	dirs := xmap.Keys(m)
	sort.Strings(dirs)
	return dirs
}

// MatchingDirs returns the list of all directories and parent directories
// hosting keys in the store's virtual filesystem.
//
// Example: []key{"a/b/c", "a/c/b"} => []dir{"a", "a/b", "a/c"}
func (t *dataStore) MatchingDirs(pattern string) ([]string, error) {
	m := make(map[string]any)
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

func (t *dataStore) AllDirs() ([]string, error) {
	return t.MatchingDirs("")
}

func (t *dataStore) AllKeys() ([]string, error) {
	return t.MatchingKeys("")
}

func (t *dataStore) MatchingKeys(pattern string) ([]string, error) {
	data := make([]string, 0)
	f := fnmatch.FNM_PATHNAME | fnmatch.FNM_LEADING_DIR

	for _, s := range t.config.Keys(dataSectionName) {
		if pattern == "" || fnmatch.Match(pattern, s, f) {
			data = append(data, s)
		}
	}
	return data, nil
}
