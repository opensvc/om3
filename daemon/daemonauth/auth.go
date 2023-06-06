package daemonauth

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/shaj13/libcache"
	_ "github.com/shaj13/libcache/fifo"

	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonctx"
	"github.com/opensvc/om3/util/key"

	"github.com/rs/zerolog/log"
	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/basic"
	"github.com/shaj13/go-guardian/v2/auth/strategies/union"
)

type (
	uxStrategy struct{}
)

var (
	Strategies union.Union
	cache      libcache.Cache
)

// User returns the logged-in user information stored in the request context.
// This func hides the go-guardian pkg from the handlers.
func User(r *http.Request) auth.Info {
	return auth.User(r)
}

func validateNode(_ context.Context, _ *http.Request, username, password string) (auth.Info, error) {
	if username == "" {
		return nil, errors.Errorf("empty user")
	}
	cluster := ccfg.Get()
	if !cluster.Nodes.Contains(username) {
		return nil, errors.Errorf("user %s is not a cluster node", username)
	}
	storedPassword := cluster.Secret()
	if storedPassword == "" {
		return nil, errors.Errorf("no cluster.secret set")
	}
	if storedPassword != password {
		return nil, errors.Errorf("wrong cluster.secret")
	}
	extensions := NewGrants("root").Extensions()
	extensions.Add("strategy", "node")
	info := auth.NewUserInfo("node-"+username, "", nil, extensions)
	return info, nil
}

func validateUser(_ context.Context, _ *http.Request, username, password string) (auth.Info, error) {
	usrPath := path.T{
		Name:      username,
		Namespace: "system",
		Kind:      kind.Usr,
	}
	usr, err := object.NewUsr(usrPath, object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	storedPassword, err := usr.DecodeKey("password")
	if err != nil {
		return nil, errors.Wrapf(err, "read password from %s", usrPath)
	}
	if string(storedPassword) != password {
		return nil, errors.Errorf("wrong password")
	}
	grants := NewGrants(usr.Config().GetStrings(key.T{Section: "DEFAULT", Option: "grant"})...)
	extensions := grants.Extensions()
	extensions.Add("strategy", "user")
	info := auth.NewUserInfo(username, "", nil, extensions)
	return info, nil
}

func (t uxStrategy) Authenticate(ctx context.Context, _ *http.Request) (auth.Info, error) {
	addr := daemonctx.ListenAddr(ctx)
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return nil, errors.Errorf("strategies/ux: is a inet address family client (%s)", addr) // How to continue ?
	}
	extensions := NewGrants("root").Extensions()
	extensions.Add("strategy", "ux")
	info := auth.NewUserInfo("root", "", nil, extensions)
	return info, nil
}

func initCache() error {
	cache = libcache.FIFO.New(0)
	cache.SetTTL(time.Minute * 5)
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

func initBasicNode() auth.Strategy {
	log.Logger.Info().Msg("init basic node auth strategy")
	basicNodeStrategy := basic.NewCached(validateNode, cache)
	return basicNodeStrategy
}

func initBasicUser() auth.Strategy {
	log.Logger.Info().Msg("init basic user auth strategy")
	basicUserStrategy := basic.NewCached(validateUser, cache)
	return basicUserStrategy
}

func initUX() auth.Strategy {
	log.Logger.Info().Msg("init ux auth strategy")
	s := &uxStrategy{}
	return s
}

func Init() error {
	if err := initCache(); err != nil {
		return err
	}
	l := make([]auth.Strategy, 0)
	for _, fn := range []func() auth.Strategy{initUX, initToken, initX509, initBasicNode, initBasicUser} {
		s := fn()
		if s == nil {
			continue
		}
		l = append(l, s)
	}
	Strategies = union.New(l...)
	return nil
}
