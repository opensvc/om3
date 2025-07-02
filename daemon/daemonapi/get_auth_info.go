package daemonapi

import (
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetAuthInfo(ctx echo.Context) error {
	config := cluster.ConfigData.Get()
	data := api.AuthInfo{
		Methods: []api.AuthInfoMethods{"basic", "x509"},
		Openid:  nil,
	}

	if config.Listener.OpenIDAuthority != "" {
		data.Methods = append(data.Methods, "openid")
		var scopes []string
		if a.OpenIDAuthority != nil {
			for _, candidate := range strings.Fields(config.Listener.OpenIDScope) {
				if slices.Contains(a.OpenIDAuthority.ScopesSupported, candidate) {
					scopes = append(scopes, candidate)
				}
			}
		}
		data.Openid = &struct {
			Authority string `json:"authority"`
			ClientId  string `json:"client_id"`
			Scope     string `json:"scope"`
		}{
			Authority: config.Listener.OpenIDAuthority,
			ClientId:  config.Listener.OpenIDClientID,
			Scope:     strings.Join(scopes, " "),
		}
	}
	return ctx.JSON(http.StatusOK, data)
}
