package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeConfigKeywords(ctx echo.Context, nodename string) error {
	r := api.KeywordDefinitionList{
		Kind:  "KeywordDefinitionList",
		Items: convertKeywordStore(object.NodeKeywordStore),
	}
	return ctx.JSON(http.StatusOK, r)
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
