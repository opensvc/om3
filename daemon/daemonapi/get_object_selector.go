package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetObjectPaths(ctx echo.Context, params api.GetObjectPathsParams) error {
	log := LogHandler(ctx, "GetObjectPaths")
	log.Debug().Msg("starting")
	paths := object.StatusData.GetPaths()
	selection := objectselector.NewSelection(
		params.Path,
		objectselector.SelectionWithInstalled(paths),
		objectselector.SelectionWithLocal(true),
	)
	matchedPaths, err := selection.Expand()
	if err != nil {
		log.Error().Err(err).Msgf("expand selection from param selector %s", params.Path)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	result := api.ObjectPaths{}
	for _, v := range matchedPaths {
		result = append(result, v.String())
	}
	return ctx.JSON(http.StatusOK, result)
}
