package resipsgcp_dnsalias

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/sgcphelper"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/httpclientcache"
	"github.com/opensvc/om3/v3/util/sgcp"
)

const noneTarget = "none."

type (
	alias struct {
		UUID   string `json:"id,omitempty"`
		Name   string `json:"name,omitempty"`
		Target string `json:"target,omitempty"`
		FQDN   string `json:"fqdn,omitempty"`
		ZoneID string `json:"zone_id,omitempty"`
	}

	aliasListResponse struct {
		Aliases []alias `json:"aliases"`
	}

	aliasManager struct {
		alias alias
		api   *sgcp.DNSAPI
	}

	T struct {
		resource.T
		resource.Restart
		datarecv.DataRecv

		UUID     string `json:"uuid,omitempty"`
		Name     string `json:"name,omitempty"`
		Target   string `json:"target,omitempty"`
		ZoneID   string `json:"zone_id,omitempty"`
		Secret   string `json:"secret,omitempty"`
		Endpoint string `json:"endpoint,omitempty"`

		api        *sgcp.DNSAPI
		configured bool
	}
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Configure() error {
	return t.configure(context.Background())
}

func (t *T) configure(ctx context.Context) error {
	if t.configured && t.api != nil {
		return nil
	}

	cfg := sgcp.GetConfig()
	if cfg == nil {
		return fmt.Errorf("mandatory sgcp config file is required: %s", sgcp.DefaultConfigPath)
	}
	if t.Target == "" {
		t.Target = hostname.Hostname()
	}
	if t.Secret == "" {
		t.Secret = cfg.GetDefaultSecret()
	}
	if t.Secret == "" {
		return errors.New("secret is required")
	}
	if t.Endpoint == "" {
		t.Endpoint = cfg.DNS.BaseURL
	}
	if t.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if t.ZoneID == "" {
		return errors.New("zone_id is required")
	}
	if t.UUID == "" && t.Name == "" {
		return errors.New("alias need define at least name or uuid")
	}

	httpClient, err := httpclientcache.Client(httpclientcache.Options{Timeout: 30 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	authInfoer := &sgcphelper.GetAuthInfoFromDatastorePather{}
	authInfo, err := authInfoer.GetAuthInfo(t.Secret)
	if err != nil {
		return fmt.Errorf("get auth info: %w", err)
	}

	tokenFactory := sgcp.NewTokenFactory(t.Log(), httpClient, &cfg.Auth, authInfo)

	config := cfg.Clone()
	if t.Endpoint != "" {
		config.DNS.BaseURL = t.Endpoint
	}
	t.api = sgcp.NewDNSAPI(config, httpClient, t.Log(), tokenFactory)
	t.configured = true
	return nil
}

func (t *T) Start(ctx context.Context) error {
	mgr := &aliasManager{alias: alias{UUID: t.UUID, Name: t.Name, Target: t.Target, ZoneID: t.ZoneID}, api: t.api}
	return mgr.createOrUpdate(ctx, t.Target)
}

func (t *T) Stop(ctx context.Context) error {
	mgr := &aliasManager{alias: alias{UUID: t.UUID, Name: t.Name, Target: t.Target, ZoneID: t.ZoneID}, api: t.api}
	if t.UUID != "" {
		return mgr.createOrUpdate(ctx, noneTarget)
	}
	return mgr.delete(ctx)
}

func (t *T) Status(ctx context.Context) status.T {
	if err := t.configure(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}

	_, _, code, data, err := t.api.GetAliases(ctx, t.ZoneID, t.Name, t.UUID)
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if code != http.StatusOK {
		t.StatusLog().Error("unexpected status code %d", code)
		return status.Undef
	}

	var resp aliasListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.StatusLog().Error("decode aliases: %s", err)
		return status.Undef
	}
	aliases := resp.Aliases

	if len(aliases) == 0 {
		t.StatusLog().Info("not found")
		return status.Down
	}
	if len(aliases) > 1 {
		t.StatusLog().Warn("found multiple aliases: %v", aliases)
		return status.Warn
	}
	found := aliases[0]
	if found.Target == noneTarget || found.Target == strings.TrimSuffix(noneTarget, ".") {
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

func (t *T) CanInstall(context.Context) (bool, error) {
	return true, nil
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}

func (m *aliasManager) createOrUpdate(ctx context.Context, target string) error {
	_, _, code, data, err := m.api.GetAliases(ctx, m.alias.ZoneID, m.alias.Name, m.alias.UUID)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", code)
	}
	var resp aliasListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode aliases: %w", err)
	}
	aliases := resp.Aliases

	if len(aliases) == 0 {
		if m.alias.UUID != "" {
			return fmt.Errorf("unable to update alias id %s, no such alias", m.alias.UUID)
		}
		_, _, code, data, err := m.api.CreateAlias(ctx, m.alias.ZoneID, m.alias.Name, target)
		if err != nil {
			return err
		}
		if code != http.StatusCreated {
			return fmt.Errorf("unexpected status code %d", code)
		}
		var created alias
		if err := json.Unmarshal(data, &created); err != nil {
			return fmt.Errorf("decode created alias: %w", err)
		}
		m.alias = created
		return nil
	}
	if len(aliases) > 1 {
		return fmt.Errorf("multiple aliases found, can't update: %v", aliases)
	}
	found := aliases[0]
	if m.alias.UUID != "" && found.UUID != "" && found.UUID != m.alias.UUID {
		return fmt.Errorf("can not update alias %v, another conflicting alias already exists: %v", m.alias, found)
	}
	_, _, code, data, err = m.api.UpdateAlias(ctx, found.ZoneID, found.UUID, m.alias.Name, target)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", code)
	}
	var updated alias
	if err := json.Unmarshal(data, &updated); err != nil {
		return fmt.Errorf("decode updated alias: %w", err)
	}
	m.alias = updated
	return nil
}

func (m *aliasManager) delete(ctx context.Context) error {
	_, _, code, data, err := m.api.GetAliases(ctx, m.alias.ZoneID, m.alias.Name, m.alias.UUID)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", code)
	}
	var resp aliasListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("decode aliases: %w", err)
	}
	aliases := resp.Aliases

	if len(aliases) == 0 {
		return nil
	}
	if len(aliases) > 1 {
		return fmt.Errorf("multiple aliases found, can't delete: %v", aliases)
	}
	_, _, code, _, err = m.api.DeleteAlias(ctx, aliases[0].ZoneID, aliases[0].UUID)
	if err != nil {
		return err
	}
	if code != http.StatusNoContent {
		return fmt.Errorf("unexpected status code %d", code)
	}
	return nil
}

func (m *aliasManager) search(ctx context.Context) ([]alias, error) {
	_, _, code, data, err := m.api.GetAliases(ctx, m.alias.ZoneID, m.alias.Name, m.alias.UUID)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", code)
	}
	var resp aliasListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode aliases: %w", err)
	}
	return resp.Aliases, nil
}
