package daemonapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

func filterKeywordStore(ctx echo.Context, store keywords.Store, driver, section, option *string, path naming.Path, getConfigProvider func() (configProvider, error)) (keywords.Store, int, error) {
	var err error
	switch {
	case driver == nil && section == nil && option == nil:
	case driver != nil && section != nil && option == nil:
		return nil, http.StatusBadRequest, fmt.Errorf("driver and section filters are mutually exclusive")
	case driver != nil && section == nil && option == nil:
		l := keywords.ParseIndex(*driver)
		store, err = store.DriverKeywords(l[0], l[1], path.Kind)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
	case driver == nil && section != nil && option == nil:
		o, err := getConfigProvider()
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		sectionType := o.Config().GetString(key.New(*section, "type"))
		drvGroup, _, _ := strings.Cut(*section, "#")
		store, err = store.DriverKeywords(drvGroup, sectionType, path.Kind)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("%s.%s: %s", drvGroup, sectionType, err)
		}
	case driver == nil && section != nil && option != nil:
		o, err := getConfigProvider()
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		sectionType := o.Config().GetString(key.New(*section, "type"))
		k := key.New(*section, *option)
		kw := store.Lookup(k, path.Kind, sectionType)
		if kw.IsZero() {
			store = []keywords.Keyword{}
		} else {
			store = []keywords.Keyword{kw}
		}
	}
	return store, http.StatusOK, nil
}

func convertKeywordStore(store keywords.Store) api.KeywordDefinitionItems {
	l := make(api.KeywordDefinitionItems, 0)
	for _, kw := range store {
		item := api.KeywordDefinitionItem{
			Option:        kw.Option,
			Section:       kw.Section,
			Default:       kw.Default,
			DefaultOption: kw.DefaultOption,
			DefaultText:   kw.DefaultText,
			Text:          kw.Text,
			Example:       kw.Example,
			Deprecated:    kw.Deprecated,
			Provisioning:  kw.Provisioning,
			Scopable:      kw.Scopable,
			Required:      kw.Required,
			Inherit:       kw.Inherit.String(),
			Aliases:       append([]string{}, kw.Aliases...),
			Candidates:    append([]string{}, kw.Candidates...),
			Types:         append([]string{}, kw.Types...),
		}

		for _, d := range kw.Depends {
			item.Depends = append(item.Depends, d.String())
		}

		for _, k := range kw.Kind {
			switch v := k.(type) {
			case string:
				item.Kind = append(item.Kind, v)
			case fmt.Stringer:
				item.Kind = append(item.Kind, v.String())
			default:
				item.Kind = append(item.Kind, fmt.Sprintf("%v", v))
			}
		}

		if kw.Converter != nil {
			item.Converter = fmt.Sprintf("%T", kw.Converter)
		}

		l = append(l, item)
	}
	return l
}
