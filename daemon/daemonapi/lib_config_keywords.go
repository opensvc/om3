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
			Option:  kw.Option,
			Section: kw.Section,
		}

		if len(kw.Aliases) > 0 {
			aliases := append([]string{}, kw.Aliases...)
			item.Aliases = &aliases
		}

		if len(kw.Candidates) > 0 {
			candidates := append([]string{}, kw.Candidates...)
			item.Candidates = &candidates
		}

		if len(kw.Depends) > 0 {
			var depends []string
			for _, d := range kw.Depends {
				depends = append(depends, d.String())
			}
			item.Depends = &depends
		}

		if len(kw.Kind) > 0 {
			var kind []string
			for _, k := range kw.Kind {
				switch v := k.(type) {
				case string:
					kind = append(kind, v)
				case fmt.Stringer:
					kind = append(kind, v.String())
				default:
					kind = append(kind, fmt.Sprintf("%v", v))
				}
			}
			item.Kind = &kind
		}

		if len(kw.Types) > 0 {
			types := append([]string{}, kw.Types...)
			item.Types = &types
		}

		if kw.Converter != nil {
			convType := fmt.Sprintf("%T", kw.Converter)
			item.Converter = &convType
		}

		if kw.Default != "" {
			item.Default = &kw.Default
		}

		if kw.DefaultOption != "" {
			item.DefaultOption = &kw.DefaultOption
		}

		if s := kw.DefaultText.String(); s != "" {
			item.DefaultText = &s
		}

		if s := kw.Text.String(); s != "" {
			item.Text = &s
		}

		if kw.Example != "" {
			item.Example = &kw.Example
		}

		if kw.Deprecated != "" {
			item.Deprecated = &kw.Deprecated
		}

		if kw.Provisioning {
			item.Provisioning = &kw.Provisioning
		}

		if kw.Scopable {
			item.Scopable = &kw.Scopable
		}
		s := kw.Inherit.String()
		item.Inherit = &s
		l = append(l, item)
	}
	return l
}
