package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetNodeConfigKeywords(ctx echo.Context, nodename string, params api.GetNodeConfigKeywordsParams) error {
	var (
		err    error
		status int
	)
	store := object.NodeKeywordStore
	path := naming.Path{}
	store, status, err = filterKeywordStore(ctx, store, params.Driver, params.Section, params.Option, path, func() (configProvider, error) {
		var (
			err error
			i   any
		)
		i, err = object.NewNode(object.WithVolatile(true))
		if err != nil {
			return nil, err
		}
		return i.(configProvider), nil
	})
	if err != nil {
		return JSONProblemf(ctx, status, http.StatusText(status), "%s", err)
	}
	r := api.KeywordDefinitionList{
		Kind:  "KeywordDefinitionList",
		Items: convertKeywordStore(store),
	}
	return ctx.JSON(http.StatusOK, r)
}
