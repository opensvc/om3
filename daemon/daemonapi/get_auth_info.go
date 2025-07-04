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

	if config.Listener.OpenIDIssuer != "" {
		data.Methods = append(data.Methods, "openid")
		data.Openid = &struct {
			ClientId string `json:"client_id"`
			Issuer   string `json:"issuer"`
		}{
			ClientId: config.Listener.OpenIDClientID,
			Issuer:   config.Listener.OpenIDIssuer,
		}
	}
	return ctx.JSON(http.StatusOK, data)
}
