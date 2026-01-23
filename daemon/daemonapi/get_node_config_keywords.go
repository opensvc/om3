package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetNodeConfigKeywords(ctx echo.Context, nodename string, params api.GetNodeConfigKeywordsParams) error {
	var err error
	store := object.NodeKeywordStore
	path := naming.Path{}
	store, err = doc.FilterKeywordStore(store, params.Driver, params.Section, params.Option, path, func() (doc.ConfigProvider, error) {
		var (
			err error
			i   any
		)
		i, err = object.NewNode(object.WithVolatile(true))
		if err != nil {
			return nil, err
		}
		return i.(doc.ConfigProvider), nil
	})
	if errors.Is(err, doc.ErrBadRequest) {
		status := http.StatusBadRequest
		return JSONProblemf(ctx, status, http.StatusText(status), "%s", err)
	} else if err != nil {
		status := http.StatusInternalServerError
		return JSONProblemf(ctx, status, http.StatusText(status), "%s", err)
	}
	r := api.KeywordDefinitionList{
		Kind:  "KeywordDefinitionList",
		Items: doc.ConvertKeywordStore(store),
	}
	return ctx.JSON(http.StatusOK, r)
}
