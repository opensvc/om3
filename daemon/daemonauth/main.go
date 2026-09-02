package daemonauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

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
		UserGranter
	}
	contextKey int

	// Info is what a strategy returns when it accepts a request: the
	// identity the rest of the daemon works with, and how it was
	// established.
	//
	// The grants are the ones the credential carries, not the ones the
	// user has: a token can be issued for a subset of its owner's grants,
	// and that subset is what is here.
	Info struct {
		// Username is who the request is from, as the rbac and the audit
		// trail name them.
		Username string

		// Strategy is the name of the strategy that accepted the
		// request, one of the Strategy* constants.
		Strategy string

		// Issuer is the node or the openid provider that issued the
		// credential, empty when it was not issued by anyone.
		Issuer string

		// TokenUse tells an access token from a refresh token from a
		// proxy token. It is empty for everything that is not a token.
		TokenUse string

		// Grants are the rbac grants the request is allowed to use.
		Grants []string
	}

	// Strategy authenticates a request, or explains why it will not.
	//
	// A strategy that does not recognize the credential a request carries
	// must return an error rather than an anonymous Info: the union tries
	// the next strategy on error, and stops at the first success.
	Strategy interface {
		Authenticate(ctx context.Context, r *http.Request) (*Info, error)
	}

	// Union is the chain of strategies the listener authenticates with.
	Union []Strategy

	StrategyManager struct {
		Mutex sync.RWMutex
		Value Union
	}
)

// AuthenticateRequest returns the Info of the first strategy that accepts
// r. When none does, it returns what each of them had to say, because
// which one was supposed to work is the caller's business, not ours: a
// client sending a bearer token learns nothing from the basic auth
// strategy saying the request has no basic auth header.
func (u Union) AuthenticateRequest(r *http.Request) (*Info, error) {
	// errors.Join of nothing is nil, and a nil error here would say the
	// request is authenticated as nobody.
	if len(u) == 0 {
		return nil, fmt.Errorf("no authentication strategy is initialized")
	}
	errs := make([]error, 0, len(u))
	for _, s := range u {
		info, err := s.Authenticate(r.Context(), r)
		if err == nil {
			return info, nil
		}
		errs = append(errs, err)
	}
	return nil, errors.Join(errs...)
}

var (
	// Strategies is the chain the listener authenticates with. It is
	// replaced, not mutated, when the auth configuration changes.
	Strategies = &StrategyManager{}

	discoverOpenIDTimeout = time.Second

	// authRefreshInterval defines the duration between periodic authentication strategy refresh operations.
	authRefreshInterval = 30 * 24 * time.Hour
)

var (
	// authCache is shared by the strategies that would otherwise redo an
	// expensive check per request. initStategies replaces it, so a
	// strategy refresh forgets what the previous configuration accepted.
	authCache = newCache()

	jwtCreatorContextKey contextKey = 1
)

const (
	StrategyUX        = "ux"
	StrategyJWT       = "jwt"
	StrategyJWTOpenID = "jwt-openid"
	StrategyUser      = "user"
	StrategyX509      = "x509"
)

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
	Strategies.setStrategy(s)
	sub := pubsub.SubFromContext(ctx, "daemon.auth")
	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})
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
						Strategies.setStrategy(s)
					}
				}
			case i := <-sub.C:
				switch c := i.(type) {
				case *msgbus.AuditStart:
					log.HandleAuditStart(c.Q, c.Subsystems, "daemonauth")
				case *msgbus.AuditStop:
					log.HandleAuditStop(c.Q, c.Subsystems, "daemonauth")
				case *msgbus.ClusterConfigUpdated:
					newSetting := signature(authCfg)
					if newSetting != currentSetting {
						log.Infof("listener setting changed, refresh authentication strategies")
						s, err := initStategies(ctx, authCfg)
						if err != nil {
							log.Errorf("failed to refresh authentication strategies: %s", err)
						} else {
							Strategies.setStrategy(s)
							currentSetting = newSetting
							ticker.Reset(authRefreshInterval)
						}
					}
				}
			}
		}
	}()

	return nil
}

// to enable all strategies, i has to implement AllStrategieser
func initStategies(ctx context.Context, i any) (Union, error) {
	authCache = newCache()
	log := plog.NewLogger(daemonlogctx.Logger(ctx)).WithPrefix("daemon: auth: ").Attr("pkg", "daemon/auth")
	l := make(Union, 0)
	for _, fn := range []func(ctx context.Context, i interface{}) (string, Strategy, error){
		initUX,
		initJWT,
		initJWTOpenID,
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
	return l, nil
}

func (m *StrategyManager) setStrategy(s Union) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.Value = s
}
