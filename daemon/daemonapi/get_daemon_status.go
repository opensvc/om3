package daemonapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
)

type (
	daemonRefresh struct {
		*sync.Mutex
		updated time.Time
	}
)

const (
	// daemonRefreshInterval defines the maximum duration before next DaemonRefresh
	daemonRefreshInterval = 2 * time.Second
)

var (
	subRefreshed = daemonRefresh{Mutex: &sync.Mutex{}}
)

// GetDaemonStatus returns daemon data status
//
// Serve 2s cached data.
func (a *DaemonAPI) GetDaemonStatus(ctx echo.Context, params api.GetDaemonStatusParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	now := time.Now()
	subRefreshed.Lock()
	if now.After(subRefreshed.updated.Add(daemonRefreshInterval)) {
		a.Daemondata.DaemonRefresh()
		subRefreshed.updated = now
	}
	subRefreshed.Unlock()

	status := a.Daemondata.ClusterData()
	if params.Selector != nil {
		status = status.WithSelector(*params.Selector)
	}
	if params.Namespace != nil {
		status = status.WithNamespace(*params.Namespace)
	}
	return ctx.JSON(http.StatusOK, status)
}
