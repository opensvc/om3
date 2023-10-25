package daemonauth

import (
	"context"
	"time"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/union"
	"github.com/shaj13/libcache"

	"github.com/opensvc/om3/util/plog"
)

var (
	cache libcache.Cache
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
)

// authenticatedExtensions returns extensions with grants and used strategy
func authenticatedExtensions(strategy string, grants ...string) *auth.Extensions {
	return &auth.Extensions{"strategy": []string{strategy}, "grant": grants}
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

// InitStategies initialize and returns strategies
// to enable all strategies, i has to implement AllStrategieser
func InitStategies(ctx context.Context, i any) (union.Union, error) {
	if err := initCache(); err != nil {
		return nil, err
	}
	log := plog.Logger{
		Logger: plog.PkgLogger(ctx, "daemon.auth"),
		Prefix: "daemon: auth: ",
	}
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
