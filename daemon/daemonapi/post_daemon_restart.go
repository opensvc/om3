package daemonapi

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/util/command"
)

func (a *DaemonApi) PostDaemonRestart(ctx echo.Context) error {
	log := LogHandler(ctx, "PostDaemonRestart")
	log.Info().Msg("starting")

	execname, err := os.Executable()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "can't detect om execname: %w", err)
	}

	cmd := command.New(
		command.WithName(execname),
		command.WithArgs([]string{"daemon", "restart", "--local"}),
	)

	err = cmd.Start()
	if err != nil {
		log.Error().Err(err).Msgf("called StartProcess")
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "daemon restart failed: %w", err)
	}
	log.Info().Msgf("called daemon restart")
	return JSONProblem(ctx, http.StatusOK, "background daemon restart has been called", "")
}
