package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/relay"
)

func (a *DaemonApi) GetRelayMessage(ctx echo.Context, params api.GetRelayMessageParams) error {
	data := api.RelayMessages{}
	if params.ClusterId != nil && params.Nodename != nil {
		if msg, ok := relay.Map.Load(*params.ClusterId, *params.Nodename); !ok {
			return JSONProblem(ctx, http.StatusNotFound, "Not found", "")
		} else {
			data.Messages = []api.RelayMessage{msg.(api.RelayMessage)}
		}
	} else {
		l := relay.Map.List()
		data.Messages = make([]api.RelayMessage, len(l))
		for i, a := range l {
			data.Messages[i] = a.(api.RelayMessage)
		}
	}
	return ctx.JSON(http.StatusOK, data)
}
