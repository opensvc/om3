package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetAuthInfo(ctx echo.Context) error {
	config := cluster.ConfigData.Get()
	data := api.AuthInfo{
		Methods: []api.AuthInfoMethods{"basic", "x509"},
	}

	if config.Listener.OpenIDAuthority != "" {
		data.Methods = append(data.Methods, "openid")
		data.Openid = &struct {
			Authority string `json:"authority"`
			ClientId  string `json:"client_id"`
		}{
			Authority: config.Listener.OpenIDAuthority,
			ClientId:  config.Listener.OpenIDClientID,
		}
	}
	return ctx.JSON(http.StatusOK, data)
}
