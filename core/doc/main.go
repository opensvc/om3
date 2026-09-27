package doc

import (
	"errors"
	"fmt"
	"slices"
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
	case driver == nil && section == nil && option != nil:
		// An option named with no section is the option in any section.
		store = store.WithOption(*option)
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

// KeywordQuery is a lookup of the keyword documentation, by section and
// option, either nil for any.
type KeywordQuery struct {
	Section *string
	Option  *string
}

// KeywordQueries returns the lookups a --kw value is tried as, in order, until
// one finds keywords.
//
// The value is [<section>.]<option>. A bare word is first an option, in any
// section, as the flag says, and then a section, listing its keywords. With
// a driver, a bare word is only an option: the driver names the section.
func KeywordQueries(s string, withDriver bool) []KeywordQuery {
	if s == "" {
		return []KeywordQuery{{}}
	}
	index := keywords.ParseIndex(s)
	if index[1] != "" {
		return []KeywordQuery{{Section: &index[0], Option: &index[1]}}
	}
	word := index[0]
	if withDriver {
		return []KeywordQuery{{Option: &word}}
	}
	return []KeywordQuery{{Option: &word}, {Section: &word}}
}

// FindKeywords returns the documentation of the keywords a --kw value names,
// from the first of its lookups finding some.
//
// A value naming no keyword is an error, rather than an empty documentation:
// nothing printed reads as a keyword with no text. A lookup failing is a
// lookup finding nothing, as a bare word that is not a section fails as a
// section, unless it is the only one.
func FindKeywords(kw string, withDriver bool, get func(section, option *string) (api.KeywordDefinitionItems, error)) (api.KeywordDefinitionItems, error) {
	queries := KeywordQueries(kw, withDriver)
	var errs error
	for _, q := range queries {
		items, err := get(q.Section, q.Option)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if len(items) > 0 || kw == "" {
			return items, nil
		}
	}
	if len(queries) == 1 && errs != nil {
		return nil, errs
	}
	return nil, fmt.Errorf("keyword %s not found", kw)
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
			Depends:       make([]string, 0, len(kw.Depends)),
			Kind:          make([]string, 0, len(kw.Kind)),
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
		sort.Strings(item.Types)

		l = append(l, item)
	}
	sort.Slice(l, func(i, j int) bool {
		left := l[i]
		right := l[j]
		if left.Section != right.Section {
			return left.Section < right.Section
		}
		if left.Option != right.Option {
			return left.Option < right.Option
		}
		if n := slices.Compare(left.Types, right.Types); n != 0 {
			return n < 0
		}
		return slices.Compare(left.Kind, right.Kind) < 0
	})
	return l
}
