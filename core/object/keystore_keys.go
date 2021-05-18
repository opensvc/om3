package object

import (
	"github.com/danwakefield/fnmatch"
)

// OptsKeys is the options of the Keys function of all data objects.
type OptsKeys struct {
	Global OptsGlobal
	Lock   OptsLocking
	Match  string `flag:"match"`
}

// Get returns a keyword value
func (t *Keystore) Keys(options OptsKeys) ([]string, error) {
	data := make([]string, 0)
	f := fnmatch.FNM_PATHNAME | fnmatch.FNM_LEADING_DIR

	for _, s := range t.config.Keys(DataSectionName) {
		if fnmatch.Match(options.Match, s, f) {
			data = append(data, s)
		}
	}
	return data, nil
}
