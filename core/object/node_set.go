package object

import (
	"opensvc.com/opensvc/core/keyop"
)

// Set gets a keyword value
func (t *Node) Set(options OptsSet) error {
	return setKeywords(t.config, options.KeywordOps)
}

func (t *Node) SetKeywords(kws []string) error {
	return setKeywords(t.config, kws)
}

func (t *Node) SetKeys(kops ...keyop.T) error {
	return setKeys(t.config, kops...)
}
