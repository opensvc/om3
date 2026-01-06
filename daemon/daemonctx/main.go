package daemonctx

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/opensvc/om3/v3/core/hbtype"
)

type (
	contextKey string
)

var (
	contextHBRecvMsgQueue        = contextKey("hb-recv-msg-queue")
	contextUUID                  = contextKey("uuid")
	contextListenAddr            = contextKey("listen-addr")
	contextLsnrType              = contextKey("lsnr-type")
	contextLsnrRateLimiterConfig = contextKey("lsnr-rate-limiter-config")
)

func (c contextKey) String() string {
	return "daemonctx." + string(c)
}

// HBRecvMsgQ function returns HBRecvMsgQ from context
// hb component send the hbrx decoded message from peers on this queue
func HBRecvMsgQ(ctx context.Context) (hbRecvQ chan<- *hbtype.Msg) {
	var ok bool
	hbRecvQ, ok = ctx.Value(contextHBRecvMsgQueue).(chan<- *hbtype.Msg)
	if ok {
		return
	}
	return nil
}

// ListenAddr function returns uuid from context
func ListenAddr(ctx context.Context) string {
	id, ok := ctx.Value(contextListenAddr).(string)
	if ok {
		return id
	}
	return ""
}

// LsnrType function returns listener family from context
func LsnrType(ctx context.Context) string {
	id, ok := ctx.Value(contextLsnrType).(string)
	if ok {
		return id
	}
	return ""
}

func ListenRateLimiterMemoryStoreConfig(ctx context.Context) middleware.RateLimiterMemoryStoreConfig {
	v, ok := ctx.Value(contextLsnrRateLimiterConfig).(middleware.RateLimiterMemoryStoreConfig)
	if ok {
		return v
	}
	return middleware.RateLimiterMemoryStoreConfig{}
}

// WithHBRecvMsgQ function returns copy of parent with HBRecvMsgQ
// the queue used by daemondata to retrieve hb rx decoded messages
func WithHBRecvMsgQ(parent context.Context, hbRecvQ chan<- *hbtype.Msg) context.Context {
	return context.WithValue(parent, contextHBRecvMsgQueue, hbRecvQ)
}

// WithLsnrType function returns copy of parent with listener family.
func WithLsnrType(parent context.Context, s string) context.Context {
	return context.WithValue(parent, contextLsnrType, s)
}

// WithUUID function returns copy of parent with uuid.
func WithUUID(parent context.Context, uuid uuid.UUID) context.Context {
	return context.WithValue(parent, contextUUID, uuid)
}

// UUID function returns uuid from context
func UUID(ctx context.Context) uuid.UUID {
	id, ok := ctx.Value(contextUUID).(uuid.UUID)
	if ok {
		return id
	}
	return uuid.UUID{}
}

// WithListenAddr function returns copy of parent with listener addr.
func WithListenAddr(parent context.Context, addr string) context.Context {
	return context.WithValue(parent, contextListenAddr, addr)
}

func WithListenRateLimiterMemoryStoreConfig(parent context.Context, rate rate.Limit, burst int, expiresIn time.Duration) context.Context {
	cfg := middleware.RateLimiterMemoryStoreConfig{Rate: rate, Burst: burst, ExpiresIn: expiresIn}
	return context.WithValue(parent, contextLsnrRateLimiterConfig, cfg)
}
