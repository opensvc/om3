package daemonauth

import (
	"context"
	"fmt"
	"sync"
	"time"

	// Build the fifo cache driver
	_ "github.com/shaj13/libcache/fifo"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/union"
	"github.com/shaj13/libcache"

	"github.com/opensvc/om3/v3/daemon/daemonlogctx"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
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

	StrategyManager struct {
		Mutex sync.RWMutex
		Value union.Union
	}
)

var (
	Strategy = &StrategyManager{}

	discoverOpenIDTimeout = time.Second

	// authRefreshInterval defines the duration between periodic authentication strategy refresh operations.
	authRefreshInterval = 30 * 24 * time.Hour
)

var (
	cache                libcache.Cache
	jwtCreatorContextKey contextKey = 1
)

const (
	StrategyUX        = "ux"
	StrategyJWT       = "jwt"
	StrategyJWTOpenID = "jwt-openid"
	StrategyNode      = "node"
	StrategyUser      = "user"
	StrategyX509      = "x509"
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

func ContextWithJWTCreator(ctx context.Context) context.Context {
	return context.WithValue(ctx, jwtCreatorContextKey, &JWTCreator{})
}

func JWTCreatorFromContext(ctx context.Context) *JWTCreator {
	return ctx.Value(jwtCreatorContextKey).(*JWTCreator)
}

func Start(ctx context.Context, authCfg any) error {
	log := plog.NewLogger(daemonlogctx.Logger(ctx)).WithPrefix("daemon: auth: ").Attr("pkg", "daemon/auth")
	signature := func(i any) string {
		cfg, ok := i.(OpenIDSettings)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s-%s", cfg.OpenIDIssuer(), cfg.OpenIDClientID())
	}

	currentSetting := signature(authCfg)

	s, err := initStategies(ctx, authCfg)
	if err != nil {
		return err
	}
	Strategy.setStrategy(s)
	sub := pubsub.SubFromContext(ctx, "daemon.auth")
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, pubsub.Label{"node", hostname.Hostname()})
	sub.Start()

	go func() {
		defer func() { _ = sub.Stop() }()
		log.Infof("starting authentication strategies routine from %s", currentSetting)

		ticker := time.NewTicker(authRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Infof("stopping authentication strategies routine")
				return
			case <-ticker.C:
				if currentSetting != "" {
					log.Infof("listener auth config refreshing strategies")
					s, err := initStategies(ctx, authCfg)
					if err != nil {
						log.Errorf("failed to refresh authentication strategies: %s", err)
					} else {
						Strategy.setStrategy(s)
					}
				}
			case <-sub.C:
				newSetting := signature(authCfg)
				if newSetting != currentSetting {
					log.Infof("listener setting changed, refresh authentication strategies")
					s, err := initStategies(ctx, authCfg)
					if err != nil {
						log.Errorf("failed to refresh authentication strategies: %s", err)
					} else {
						Strategy.setStrategy(s)
						currentSetting = newSetting
						ticker.Reset(authRefreshInterval)
					}
				}
			}
		}
	}()

	return nil
}

// to enable all strategies, i has to implement AllStrategieser
func initStategies(ctx context.Context, i any) (union.Union, error) {
	if err := initCache(); err != nil {
		return nil, err
	}
	log := plog.NewLogger(daemonlogctx.Logger(ctx)).WithPrefix("daemon: auth: ").Attr("pkg", "daemon/auth")
	l := make([]auth.Strategy, 0)
	for _, fn := range []func(ctx context.Context, i interface{}) (string, auth.Strategy, error){
		initUX,
		initJWT,
		initJWTOpenID,
		initBasicNode,
		initBasicUser,
		initX509,
	} {
		name, s, err := fn(ctx, i)
		if err != nil {
			log.Warnf("ignored authentication strategy %s: %s", name, err)
		} else if s != nil {
			log.Infof("initialized authentication strategy %s", name)
			if name == "jwt" {
				log.Infof("jwt verify key sig: %s", jwtVerifyKeySign)
			}
			l = append(l, s)
		}
	}
	return union.New(l...), nil
}

func (m *StrategyManager) setStrategy(s union.Union) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.Value = s
}
