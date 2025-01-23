package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/shaj13/go-guardian/v2/auth"

	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonauth"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	JWTCreater interface {
		CreateUserToken(userInfo auth.Info, duration time.Duration, xClaims map[string]interface{}) (tk string, expiredAt time.Time, err error)
	}

	DaemonAPI struct {
		Daemondata *daemondata.T
		EventBus   *pubsub.Bus
		JWTcreator JWTCreater

		LabelLocalhost pubsub.Label

		localhost string
		SubQS     pubsub.QueueSizer
	}

	contextKey string
)

var (
	contextApiSubQS = contextKey("api-sub-queue-size")

	labelOriginAPI = pubsub.Label{"origin", "api"}
)

func New(ctx context.Context) *DaemonAPI {
	localhost := hostname.Hostname()
	return &DaemonAPI{
		Daemondata:     daemondata.FromContext(ctx),
		EventBus:       pubsub.BusFromContext(ctx),
		JWTcreator:     daemonauth.JWTCreatorFromContext(ctx),
		LabelLocalhost: pubsub.Label{"node", localhost},
		localhost:      localhost,
		SubQS:          SubQS(ctx),
	}
}

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

func JSONForbiddenMissingGrant(ctx echo.Context, missing ...rbac.Grant) error {
	return JSONProblemf(ctx, http.StatusForbidden, "Missing grants", "not allowed, need one of %v grant", missing)
}

func JSONForbiddenMissingRole(ctx echo.Context, missing ...rbac.Role) error {
	return JSONProblemf(ctx, http.StatusForbidden, "Missing grants", "not allowed, need one of %v role", missing)
}

func JSONForbiddenStrategy(ctx echo.Context, strategy string, expected ...string) error {
	return JSONProblemf(ctx, http.StatusForbidden, "Unexpected strategy", "not allowed strategy %s, need one of %v strategy", strategy, expected)
}

func setStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}

// SubQS function returns api pubsub.QueueSizer from context
func SubQS(ctx context.Context) pubsub.QueueSizer {
	subQS, ok := ctx.Value(contextApiSubQS).(pubsub.QueueSizer)
	if ok {
		return subQS
	}
	return pubsub.WithQueueSize(daemonenv.SubQSLarge)
}

// WithSubQS function returns copy of parent with api pubsub.QueueSizer.
func WithSubQS(parent context.Context, subQS pubsub.QueueSizer) context.Context {
	return context.WithValue(parent, contextApiSubQS, subQS)
}
