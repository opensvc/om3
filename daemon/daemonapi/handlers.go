package daemonapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/daemon/subdaemon"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	JWTCreater interface {
		CreateUserToken(userInfo auth.Info, duration time.Duration, xClaims map[string]interface{}) (tk string, expiredAt time.Time, err error)
	}

	DaemonApi struct {
		Daemon     subdaemon.RootManager
		Daemondata *daemondata.T
		EventBus   *pubsub.Bus
		JWTcreator JWTCreater
	}
)

var (
	labelApi  = pubsub.Label{"origin", "api"}
	labelNode = pubsub.Label{"node", hostname.Hostname()}
)

func JSONProblem(ctx echo.Context, code int, title, detail string) error {
	return ctx.JSON(code, api.Problem{
		Detail: detail,
		Title:  title,
		Status: code,
	})
}

func JSONProblemf(ctx echo.Context, code int, title, detail string, argv ...any) error {
	return ctx.JSON(code, api.Problem{
		Detail: fmt.Sprintf(detail, argv...),
		Title:  title,
		Status: code,
	})
}

func JSONForbiddenMissingRole(ctx echo.Context, missing ...rbac.Role) error {
	return JSONProblemf(ctx, http.StatusForbidden, "Missing grants", "not allowed, need one of %v role", missing)
}

func setStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}
