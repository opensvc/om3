package daemonapi

import (
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/daemon/api"
	"net/http"
)

func (a *DaemonAPI) GetAuthInfo(ctx echo.Context) error {
	config := clusterdump.ConfigData.Get()
	data := api.AuthInfo{
		Methods: []api.AuthInfoMethods{"basic", "x509"},
		Openid:  nil,
	}

	if config.Listener.OpenIDWellKnown != "" {
		data.Methods = append(data.Methods, "openid")
		data.Openid = &struct {
			ClientId     string `json:"client_id"`
			WellKnownUri string `json:"well_known_uri"`
		}{
			ClientId:     config.Name,
			WellKnownUri: config.Listener.OpenIDWellKnown,
		}
	}
	return ctx.JSON(http.StatusOK, data)
}
