package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostDaemonLogControl(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	var payload api.LogControl
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "error: %s", err)
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.postLocalDaemonLogControl(ctx, payload)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonLogControl(ctx.Request().Context(), nodename, payload)
	})
}

func (a *DaemonAPI) postLocalDaemonLogControl(ctx echo.Context, payload api.LogControl) error {
	var level string
	switch payload.Level {
	case "":
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "missing 'level' field")
	case "none":
		// NoLevel
	default:
		level = payload.Level
	}
	newLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing 'level': %s", err)
	}
	// The daemon writes to journald through a writer that drops what is
	// below info, so a lower level here would set a global nothing can
	// honor: the request would succeed and change nothing. The debug and
	// trace feed is the audit endpoint, which reads the messages before
	// they reach a writer.
	if newLevel < zerolog.InfoLevel {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body",
			"the daemon log is not emitted below the info level: use the daemon audit endpoint for a %s feed", payload.Level)
	}
	zerolog.SetGlobalLevel(newLevel)
	return ctx.JSON(http.StatusOK, api.LogControl{Level: logControlLevel()})
}

// GetDaemonLogControl reports the level the daemon logs are emitted at.
func (a *DaemonAPI) GetDaemonLogControl(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return ctx.JSON(http.StatusOK, api.LogControl{Level: logControlLevel()})
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetDaemonLogControl(ctx.Request().Context(), nodename)
	})
}

// logControlLevel returns the current global level, in the spelling the
// api takes and returns. Zerolog spells the level that emits nothing
// "", which is the spelling the body uses for "no level was asked for".
func logControlLevel() string {
	if level := zerolog.GlobalLevel(); level < zerolog.NoLevel {
		return level.String()
	}
	return "none"
}
