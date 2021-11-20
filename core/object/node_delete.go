package object

import (
	"strings"
)

//
// Delete is the 'delete' node action entrypoint.
//
// If a resource selector is set, only delete the corresponding
// sections in the configuration file.
//
func (t Node) Delete(opts OptsDelete) error {
	if opts.ResourceSelector != "" {
		return t.deleteSections(opts.ResourceSelector)
	}
	return nil
}

func (t Node) deleteSections(s string) error {
	sections := strings.Split(s, ",")
	return t.config.DeleteSections(sections)
}
