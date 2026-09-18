package sgcphelper

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/sgcp"
)

const (
	// CacheVar is the environment variable that forces the sgcp drivers
	// cache policy whatever the action origin: "1" serves the cached values,
	// "0" never does. An unset variable leaves the default policy.
	CacheVar = "OSVC_SGCP_CACHE"
)

// UseCache tells whether a driver can serve what it cached instead of
// reading the provider again. By default, only the daemon scheduler is
// served the cache: its status evaluations run over and over on their own,
// and the cache is what keeps them off the provider api. Everyone else, an
// operator asking for a status first of all, is asking what the provider
// says now.
//
// OSVC_SGCP_CACHE overrides this default: "1" serves the cache to every
// action origin, "0" serves it to none, the daemon scheduler included.
// Either way, a cached value is only served while younger than the sgcp
// cache.ttl_seconds, and a zero ttl disables the cache.
//
// Any other value of OSVC_SGCP_CACHE leaves the default policy, and is
// reported by the returned error so the caller can tell the operator
// the setting is ignored.
//
// This lives here rather than in util/sgcp because it reads the action
// origin, and a util package does not depend on core.
func UseCache() (bool, error) {
	switch v := os.Getenv(CacheVar); v {
	case "1":
		return true, nil
	case "0":
		return false, nil
	case "":
		return env.HasDaemonSchedulerOrigin(), nil
	default:
		return env.HasDaemonSchedulerOrigin(), fmt.Errorf("ignored %s=%q: expected 0 or 1", CacheVar, v)
	}
}

type (
	GetAuthInfoFromDatastorePather struct{}
)

func AuthInfoFromPath(s string) (*sgcp.AuthInfo, error) {
	t := &GetAuthInfoFromDatastorePather{}
	return t.GetAuthInfo(s)
}

// GetAuthInfo retrieves authentication information from the specified datastore path and returns an AuthInfo struct.
func (g *GetAuthInfoFromDatastorePather) GetAuthInfo(datastorePath string) (*sgcp.AuthInfo, error) {
	var (
		ds       object.DataStore
		dsValues [][]byte
	)
	secPath, err := naming.ParsePath(datastorePath)
	if err != nil {
		return nil, fmt.Errorf("parse secret path: %w", err)
	}

	ds, err = object.NewSec(secPath, object.WithVolatile(true))
	if err != nil {
		return nil, fmt.Errorf("get secret %s: %w", secPath, err)
	}

	dsValues, err = ds.DecodeKeys("account_id", "client_id", "client_secret")
	if err != nil {
		return nil, fmt.Errorf("decode auth keys from %s: %w", secPath, err)
	}
	authInfo := &sgcp.AuthInfo{
		AccountID:    string(dsValues[0]),
		ClientID:     string(dsValues[1]),
		ClientSecret: string(dsValues[2]),

		Signature: fmt.Sprintf("sgcp-authinfo-%s-%s", secPath.Namespace, secPath.Name),
	}
	return authInfo, nil
}
