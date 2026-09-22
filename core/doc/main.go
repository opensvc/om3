package doc

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	ConfigProvider interface {
		Config() *xconfig.T
	}
)

var (
	ErrBadRequest = errors.New("driver and section filters are mutually exclusive")
)

func FilterKeywordStore(store keywords.Store, driver, section, option *string, path naming.Path, getConfigProvider func() (ConfigProvider, error)) (keywords.Store, error) {
	var err error
	switch {
	case driver == nil && section == nil && option == nil:
	case driver != nil && section != nil && option == nil:
		return nil, ErrBadRequest
	case driver != nil && section == nil && option == nil:
		l := keywords.ParseIndex(*driver)
		store, err = store.DriverKeywords(l[0], l[1], path.Kind)
		if err != nil {
			return nil, err
		}
	case driver != nil && option != nil:
		l := keywords.ParseIndex(*driver)
		store, err = store.DriverKeywords(l[0], l[1], path.Kind)
		if *option == "" && section != nil {
			return store.ByOption(*section), nil
		}
		return store.ByOption(*option), nil
	case driver == nil && section != nil && option == nil:
		o, err := getConfigProvider()
		if err != nil {
			return nil, err
		}
		sectionType := o.Config().GetString(key.New(*section, "type"))
		drvGroup, _, _ := strings.Cut(*section, "#")
		store, err = store.DriverKeywords(drvGroup, sectionType, path.Kind)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %s", drvGroup, sectionType, err)
		}
	case driver == nil && section != nil && option != nil:
		o, err := getConfigProvider()
		if err != nil {
			return nil, err
		}
		sectionType := o.Config().GetString(key.New(*section, "type"))
		k := key.New(*section, *option)
		kw := o.Config().Referrer.KeywordLookup(k, sectionType)
		if kw == nil {
			store = []*keywords.Keyword{}
		} else {
			store = []*keywords.Keyword{kw}
		}
	}
	return store, nil
}

func ConvertKeywordStore(store keywords.Store) api.KeywordDefinitionItems {
	l := make(api.KeywordDefinitionItems, 0)
	for _, kw := range store {
		item := api.KeywordDefinitionItem{
			Option:        kw.Option,
			Section:       kw.Section,
			Converter:     converters.Name(kw.Converter),
			Default:       kw.Default,
			DefaultOption: kw.DefaultOption,
			DefaultText:   kw.DefaultText,
			Text:          kw.Text,
			Example:       kw.Example,
			Since:         kw.Since,
			Deprecated:    kw.Deprecated,
			ReplacedBy:    kw.ReplacedBy,
			RedactSecret:  kw.RedactSecret,
			Recorded:      kw.Recorded,
			Arithmetic:    kw.Arithmetic,
			Provisioning:  kw.Provisioning,
			Scopable:      kw.Scopable,
			Minimal:       kw.Minimal,
			Required:      kw.Required,
			Inherit:       kw.Inherit.String(),
			Aliases:       append([]string{}, kw.Aliases...),
			Candidates:    append([]string{}, kw.Candidates...),
			Types:         append([]string{}, kw.Types...),
		}

		for _, d := range kw.Depends {
			item.Depends = append(item.Depends, d.String())
		}

		// The kinds a keyword is limited to are held as a set, whose values
		// are nil and whose keys are the kinds: reading the values, as this
		// did, documented every such keyword as scoped to "<nil>". The keys
		// are sorted, a set having no order of its own and this being
		// compared from one release to the next.
		for kind := range kw.Kind {
			item.Kind = append(item.Kind, kind.String())
		}
		sort.Strings(item.Kind)

		l = append(l, item)
	}
	return l
}
