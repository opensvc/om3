package sgcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/opensvc/om3/v3/util/plog"
)

// DNSAPI provides methods for DNS alias management.
type DNSAPI struct {
	Api
	config *Config
}

// Alias represents a DNS alias.
type Alias struct {
	UUID   string `json:"id,omitempty"`
	Name   string `json:"name,omitempty"`
	Target string `json:"target,omitempty"`
	FQDN   string `json:"fqdn,omitempty"`
	ZoneID string `json:"zoneId,omitempty"`
}

// NewDNSAPI creates a new DNSAPI instance.
func NewDNSAPI(config *Config, client *http.Client, l *plog.Logger, tk tokenGetter) *DNSAPI {
	return &DNSAPI{
		config: config,
		Api: Api{
			client: client,
			tk:     tk,
			log:    l,
		},
	}
}

// GetAliases retrieves aliases matching the given criteria.
func (a *DNSAPI) GetAliases(ctx context.Context, zoneID, name, uuid string) (method, url string, code int, data []byte, err error) {
	method = http.MethodGet
	url = a.getAliasesURL(zoneID, name, uuid)
	code, data, err = a.do(ctx, method, url, nil, a.GetScopes("dns_read")...)
	return
}

// CreateAlias creates a new DNS alias.
func (a *DNSAPI) CreateAlias(ctx context.Context, zoneID, name, target string) (alias *Alias, err error) {
	var result Alias
	method := http.MethodPost
	path := a.getAliasesCreateURL(zoneID)

	payload := map[string]any{"name": name, "target": target, "ttl": 60}
	var b []byte
	b, err = json.Marshal(payload)
	if err != nil {
		err = fmt.Errorf("failed to marshal create payload: %w", err)
		return
	}
	a.log.Infof("%s %s data=%s", method, path, string(b))
	if code, data, err := a.do(ctx, method, path, bytes.NewReader(b), a.GetScopes("dns_write")...); err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	} else if err := a.CheckStatusCode(method, path, code, http.StatusCreated); err != nil {
		return nil, err
	} else if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("%s %s unmarshal created alias: %w", method, path, err)
	}
	return &result, nil
}

// UpdateAlias updates an existing DNS alias (PATCH, target only).
func (a *DNSAPI) UpdateAlias(ctx context.Context, zoneID, aliasUUID, name, target string) (alias *Alias, err error) {
	method := http.MethodPatch
	path := a.getAliasURL(zoneID, aliasUUID)

	payload := map[string]any{"target": target, "ttl": 60}
	var b []byte
	b, err = json.Marshal(payload)
	if err != nil {
		err = fmt.Errorf("failed to marshal update payload: %w", err)
		return
	}
	a.log.Infof("%s %s data=%s", method, path, string(b))
	if code, data, err := a.do(ctx, method, path, bytes.NewReader(b), a.GetScopes("dns_write")...); err != nil {
		return nil, err
	} else if err := a.CheckStatusCode(method, path, code, http.StatusOK); err != nil {
		return nil, err
	} else if err := json.Unmarshal(data, &alias); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alias: %w", err)
	}
	return
}

// DeleteAlias deletes a DNS alias.
func (a *DNSAPI) DeleteAlias(ctx context.Context, zoneID, aliasUUID string) error {
	method := http.MethodDelete
	path := a.getAliasURL(zoneID, aliasUUID)

	a.log.Infof("%s %s", method, path)
	if code, _, err := a.do(ctx, method, path, nil, a.GetScopes("dns_write")...); err != nil {
		return err
	} else if err := a.CheckStatusCode(method, path, code, http.StatusNoContent); err != nil {
		return err
	}
	return nil
}

// GetScopes returns the scopes for a given scope type.
func (a *DNSAPI) GetScopes(scopeType string) []string {
	return a.config.GetScopes(scopeType)
}

// getAliasesURL constructs the URL for listing aliases with query parameters.
func (a *DNSAPI) getAliasesURL(zoneID, name, uuid string) string {
	values := url.Values{}
	if name != "" {
		values.Set("name", name)
	}
	if uuid != "" {
		values.Set("id", uuid)
	}
	base := strings.TrimRight(a.config.DNS.BaseURL, "/")
	zonePath := a.config.DNS.Path.Zone
	aliasPath := a.config.DNS.Path.CName
	return fmt.Sprintf("%s%s/%s%s?%s", base, zonePath, zoneID, aliasPath, values.Encode())
}

// getAliasesCreateURL constructs the URL for creating an alias.
func (a *DNSAPI) getAliasesCreateURL(zoneID string) string {
	base := strings.TrimRight(a.config.DNS.BaseURL, "/")
	zonePath := a.config.DNS.Path.Zone
	aliasPath := a.config.DNS.Path.CName
	return fmt.Sprintf("%s%s/%s%s", base, zonePath, zoneID, aliasPath)
}

// getAliasURL constructs the URL for a specific alias.
func (a *DNSAPI) getAliasURL(zoneID, aliasUUID string) string {
	base := strings.TrimRight(a.config.DNS.BaseURL, "/")
	zonePath := a.config.DNS.Path.Zone
	aliasPath := a.config.DNS.Path.CName
	return fmt.Sprintf("%s%s/%s%s/%s", base, zonePath, zoneID, aliasPath, aliasUUID)
}
