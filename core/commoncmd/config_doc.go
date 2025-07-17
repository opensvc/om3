package commoncmd

import (
	"io"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func KeywordStoreFromAPI(items api.KeywordDefinitionItems) (store keywords.Store) {
	for _, item := range items {
		kw := keywords.Keyword{
			Converter:     item.Converter,
			Default:       item.Default,
			DefaultOption: item.DefaultOption,
			DefaultText:   item.DefaultText,
			Depends:       keyop.ParseList(item.Depends...),
			Deprecated:    item.Deprecated,
			Example:       item.Example,
			Inherit:       keywords.ParseInherit(item.Inherit),
			Kind:          naming.ParseKinds(item.Kind...),
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

func Doc(w io.Writer, items api.KeywordDefinitionItems, kind naming.Kind, depth int) error {
	store := KeywordStoreFromAPI(items)
	return store.Doc(w, kind, depth)
}
