package commoncmd

import (
	"io"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/keyoprbac"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/converters"
)

func KeywordStoreFromAPI(items api.KeywordDefinitionItems) (store keywords.Store) {
	for _, item := range items {
		// A peer daemon may run a different version, exposing a converter
		// name this version does not know. Fall back to no conversion
		// instead of failing.
		converter, _ := converters.Get(item.Converter)
		kw := &keywords.Keyword{
			Converter:     converter,
			Default:       item.Default,
			DefaultOption: item.DefaultOption,
			DefaultText:   item.DefaultText,
			Depends:       keyop.ParseList(item.Depends...),
			Deprecated:    item.Deprecated,
			ReplacedBy:    item.ReplacedBy,
			RedactSecret:  item.RedactSecret,
			Example:       item.Example,
			Inherit:       keywords.ParseInherit(item.Inherit),
			Kind:          naming.ParseKinds(item.Kind...),
			Minimal:       item.Minimal,
			Option:        item.Option,
			Provisioning:  item.Provisioning,
			Required:      item.Required,
			Scopable:      item.Scopable,
			Section:       item.Section,
			Text:          item.Text,
		}
		kw.Aliases = append(kw.Aliases, item.Aliases...)
		kw.Candidates = append(kw.Candidates, item.Candidates...)
		kw.Types = append(kw.Types, item.Types...)
		store = append(store, kw)
	}
	return
}

// Doc renders the documentation of the keywords of an object configuration.
//
// The keywords carry what the rbac policy says about them, because an object
// configuration is what a user without the root grant sends to the api, and
// the policy decides which of its keywords the api accepts.
func Doc(w io.Writer, items api.KeywordDefinitionItems, kind naming.Kind, driver, kw string, depth int) error {
	store := KeywordStoreFromAPI(items)
	return store.Doc(w, kind, driver, kw, depth, keyoprbac.Doc)
}

// NodeDoc renders the documentation of the keywords of the node configuration.
//
// They carry no rbac line: the node configuration is not written through the
// api by a user holding a grant on a namespace, so the policy that gates the
// object configurations says nothing about them.
func NodeDoc(w io.Writer, items api.KeywordDefinitionItems, kind naming.Kind, driver, kw string, depth int) error {
	store := KeywordStoreFromAPI(items)
	return store.Doc(w, kind, driver, kw, depth, nil)
}
