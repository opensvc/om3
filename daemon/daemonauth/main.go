package daemonauth

import (
	"context"
	"time"

	// Build the fifo cache driver
	_ "github.com/shaj13/libcache/fifo"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/union"
	"github.com/shaj13/libcache"

	"github.com/opensvc/om3/daemon/daemonlogctx"
	"github.com/opensvc/om3/util/plog"
)

type (
	// AllStrategieser defines interfaces that allows all strategies
	AllStrategieser interface {
		ListenAddresser
		JWTFiler
		X509CACertFiler
		NodeAuthenticater
		UserGranter
	}
	contextKey int
)

var (
	cache                libcache.Cache
	strategiesContextKey contextKey = 0
	jwtCreatorContextKey contextKey = 1
)

const (
	StrategyUX   = "ux"
	StrategyJWT  = "jwt"
	StrategyNode = "node"
	StrategyUser = "user"
	StrategyX509 = "x509"
)

// authenticatedExtensions returns extensions with grants and used strategy
func authenticatedExtensions(strategy string, iss string, grants ...string) *auth.Extensions {
	extensions := auth.Extensions{"strategy": []string{strategy}, "grant": grants}
	if iss != "" {
		extensions.Set("iss", iss)
	}
	return &extensions
}

func initCache() error {
	cache = libcache.FIFO.New(0)
	cache.SetTTL(time.Second * 5)
	/*
		q := make(chan libcache.Event)
		cache.Notify(q, libcache.Remove)
		go func() {
			for {
				select {
				case ev := <-q:
					cache.Peek(ev.Key)
				}
			}
		}()
	*/
	return nil
}

func ContextWithStrategies(ctx context.Context, strategies union.Union) context.Context {
	return context.WithValue(ctx, strategiesContextKey, strategies)
}

func StrategiesFromContext(ctx context.Context) union.Union {
	return ctx.Value(strategiesContextKey).(union.Union)
}

func ContextWithJWTCreator(ctx context.Context) context.Context {
	return context.WithValue(ctx, jwtCreatorContextKey, &JWTCreator{})
}

func JWTCreatorFromContext(ctx context.Context) *JWTCreator {
	return ctx.Value(jwtCreatorContextKey).(*JWTCreator)
}

// InitStategies initialize and returns strategies
// to enable all strategies, i has to implement AllStrategieser
func InitStategies(ctx context.Context, i any) (union.Union, error) {
	if err := initCache(); err != nil {
		return nil, err
	}
	log := plog.NewLogger(daemonlogctx.Logger(ctx)).WithPrefix("daemon: auth: ").Attr("pkg", "daemon/auth")
	l := make([]auth.Strategy, 0)
	for _, fn := range []func(i interface{}) (string, auth.Strategy, error){
		initUX,
		initJWT,
		initX509,
		initBasicNode,
		initBasicUser,
	} {
		name, s, err := fn(i)
		if err != nil {
			log.Errorf("init strategy %s error: %s", name, err)
		} else {
			log.Infof("init strategy %s", name)
			if name == "jwt" {
				log.Infof("jwt verify key sig: %s", jwtVerifyKeySign)
			}
			l = append(l, s)
		}
	}
	return union.New(l...), nil
}
