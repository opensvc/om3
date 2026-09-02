package resipsgcp_dnsalias

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/sgcphelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/httpclientcache"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sgcp"
)

type (
	T struct {
		resource.T
		resource.Restart

		UUID     string `json:"uuid,omitempty"`
		Name     string `json:"name,omitempty"`
		Target   string `json:"target,omitempty"`
		ZoneID   string `json:"zone_id,omitempty"`
		Secret   string `json:"secret,omitempty"`
		Endpoint string `json:"endpoint,omitempty"`

		mgr *mgr

		// for tests
		api        apiProvider
		noneTarget string
	}

	mgr struct {
		alias    alias
		api      apiProvider
		log      *plog.Logger
		CacheTTL time.Duration
		Endpoint string
		Secret   string
	}

	// alias decoupled from sgcp.Alias to allow for future changes
	alias struct {
		UUID   string `json:"id,omitempty"`
		Name   string `json:"name,omitempty"`
		Target string `json:"target,omitempty"`
		FQDN   string `json:"fqdn,omitempty"`
		ZoneID string `json:"zone_id,omitempty"`
	}

	aliasListResponse struct {
		CnameRecords []sgcp.Alias `json:"cnameRecords"`
	}

	apiProvider interface {
		CheckStatusCode(method string, url string, got int, wanted ...int) error
		CreateAlias(ctx context.Context, zoneID, name, target string) (alias *sgcp.Alias, err error)
		DeleteAlias(ctx context.Context, zoneID, aliasUUID string) (err error)
		GetAliases(ctx context.Context, zoneID, name, uuid string) (method, url string, code int, data []byte, err error)
		UpdateAlias(ctx context.Context, zoneID string, aliasUUID string, name string, target string) (alias *sgcp.Alias, err error)
	}
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Configure() error {
	cfg := sgcp.GetConfig()
	if cfg == nil {
		return fmt.Errorf("mandatory sgcp config file is required: %s", sgcp.DefaultConfigPath)
	}
	if t.Target == "" {
		t.Target = hostname.Hostname()
	}

	t.noneTarget = cfg.DNS.NoneTarget
	if t.noneTarget == "" {
		return fmt.Errorf("dns.none_target is required in sgcp config")
	}

	// secret is mandatory: define from keyword, fallback to cfg default, ensure not empty
	if t.Secret == "" {
		t.Secret = cfg.GetDefaultSecret()
	}
	if t.Secret == "" {
		return errors.New("secret is required")
	}

	// endpoint is mandatory: define from keyword, fallback to cfg default, ensure not empty
	if t.Endpoint == "" {
		t.Endpoint = cfg.DNS.BaseURL
	}
	if t.Endpoint == "" {
		return errors.New("endpoint is required")
	}

	// zoneid is mandatory
	if t.ZoneID == "" {
		return errors.New("zone_id is required")
	}
	if t.UUID == "" && t.Name == "" {
		return errors.New("alias need define at least name or uuid")
	}

	return t.configureMgr(cfg)
}

func (t *T) configureMgr(cfg *sgcp.Config) error {
	mgr := &mgr{
		alias:    alias{UUID: t.UUID, Name: t.Name, Target: t.Target, ZoneID: t.ZoneID},
		log:      t.Log(),
		CacheTTL: time.Duration(cfg.Cache.TTLSeconds) * time.Second,
		Endpoint: t.Endpoint,
		Secret:   t.Secret,
	}
	if t.api != nil {
		// allow custom api for tests
		mgr.api = t.api
		t.mgr = mgr
		return nil
	}

	httpClient, err := httpclientcache.Client(httpclientcache.Options{Timeout: 30 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	authInfo, err := sgcphelper.AuthInfoFromPath(t.Secret)
	if err != nil {
		return fmt.Errorf("get auth info: %w", err)
	}

	tokenFactory := sgcp.NewTokenFactory(t.Log(), httpClient, &cfg.Auth, authInfo)

	mgr.api = sgcp.NewDNSAPI(cfg, httpClient, t.Log(), tokenFactory)

	t.mgr = mgr
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if disabled, err := t.isDisabled(); err != nil {
		return err
	} else if disabled {
		t.Log().Infof("skip start of alias %s: sgcp support disabled", t.Name)
		return nil
	}
	return t.mgr.createOrUpdate(ctx, t.Target)
}

func (t *T) Stop(ctx context.Context) error {
	if disabled, err := t.isDisabled(); err != nil {
		return err
	} else if disabled {
		t.Log().Infof("skip stop of alias %s: sgcp support disabled", t.Name)
		return nil
	}
	if t.UUID != "" {
		return t.mgr.createOrUpdate(ctx, t.noneTarget)
	}
	return t.mgr.delete(ctx)
}

// isDisabled tells whether the operator has disabled the sgcp support. An
// undecidable flag is an error the actions report, rather than a reason to
// quietly skip the work they were asked to do.
func (t *T) isDisabled() (bool, error) {
	return sgcp.IsDisabled(rawconfig.NodeVarDir())
}

func (t *T) Status(ctx context.Context) status.T {
	if disabled, err := t.isDisabled(); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.NotApplicable
	} else if disabled {
		t.StatusLog().Info("xaas status disabled")
		return status.NotApplicable
	}
	aliases, err := t.mgr.getAliases(ctx)
	if err != nil {
		t.StatusLog().Error("get alias failed: %s", err)
	}

	if len(aliases) == 0 {
		t.StatusLog().Info("not found")
		return status.Down
	}
	if len(aliases) > 1 {
		t.StatusLog().Warn("found multiple aliases: %v", aliases)
		return status.Warn
	}
	found := aliases[0]
	if found.Target == t.noneTarget || found.Target == strings.TrimSuffix(t.noneTarget, ".") {
		t.StatusLog().Info("alias target is disabled")
		return status.Down
	}
	if t.Name != "" && found.Name != t.Name {
		t.StatusLog().Warn("alias name mismatch: found %s instead of %s", found.Name, t.Name)
		return status.Warn
	}
	if found.Target != t.Target {
		t.StatusLog().Info("alias target %s != %s", found.Target, t.Target)
		return status.Down
	}
	if t.UUID != "" && found.UUID != "" && found.UUID != t.UUID {
		t.StatusLog().Warn("alias uuid mismatch: found %s instead of %s", found.UUID, t.UUID)
		return status.Warn
	}
	t.StatusLog().Info("%s => %s", found.FQDN, found.Target)
	return status.Up
}

func (t *T) Label(context.Context) string {
	if t.Name != "" {
		return t.Name
	}
	if t.UUID != "" {
		return t.UUID
	}
	return t.ZoneID
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}
