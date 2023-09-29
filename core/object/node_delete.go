package object

import (
	"strings"
)

// DeleteSection is the 'delete' node action entrypoint.
//
// If a resource selector is set, only delete the corresponding
// sections in the configuration file.
func (t Node) DeleteSection(s string) error {
	sections := strings.Split(s, ",")
	return t.config.DeleteSections(sections)
}
