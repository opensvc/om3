package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// ErrDiscovering is the error of an object listing asked while the daemon
// has not discovered the objects of its configuration files yet: an empty
// list would say there are none (card 308).
var ErrDiscovering = errors.New("the daemon is starting: the local objects are not discovered yet")

// JSONProblemExpand answers the error of a selection expansion: a 503 while
// the objects are not discovered, the client to ask again or to read its
// local files, a 500 otherwise.
func JSONProblemExpand(ctx echo.Context, err error) error {
	if errors.Is(err, ErrDiscovering) {
		return JSONProblem(ctx, http.StatusServiceUnavailable, "Starting", ErrDiscovering.Error())
	}
	return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
}
