package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetObjectConfigKeywords(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	r := api.KeywordDefinitionList{
		Kind:  "KeywordDefinitionList",
		Items: convertKeywordStore(object.KeywordStoreWithDrivers(kind)),
	}
	return ctx.JSON(http.StatusOK, r)
}
