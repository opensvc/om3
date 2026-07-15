package resipsgcp_dnsalias

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/util/sgcp"
)

var (
	ErrUpdateNotFound = fmt.Errorf("update: alias not found")
)

func (m *mgr) createOrUpdate(ctx context.Context, target string) error {
	aliases, err := m.getAliases(ctx)
	if err != nil {
		return fmt.Errorf("get aliases: %w", err)
	}

	if len(aliases) == 0 {
		if m.alias.UUID != "" {
			return fmt.Errorf("uuid %s: %w", m.alias.UUID, ErrUpdateNotFound)
		}
		if v, err := m.create(ctx, target); err != nil {
			return fmt.Errorf("create alias: %w", err)
		} else {
			m.alias = *v
			return nil
		}
	}
	if len(aliases) > 1 {
		return fmt.Errorf("multiple aliases found, can't update: %v", aliases)
	}
	found := aliases[0]
	if m.alias.UUID != "" && found.UUID != "" && found.UUID != m.alias.UUID {
		return fmt.Errorf("can not update alias %v, another conflicting alias already exists: %v", m.alias, found)
	}

	expectedAlias := m.alias
	expectedAlias.Target = target
	if expectedAlias.Equal(toAlias(&found)) {
		m.log.Debugf("alias already exists")
		return nil
	}

	if v, err := m.update(ctx, found.ZoneID, found.UUID, m.alias.Name, target); err != nil {
		return fmt.Errorf("update alias: %w", err)
	} else if v == nil {
		return fmt.Errorf("update alias unexpected nil")
	} else {
		m.alias = *v
		return nil
	}
}

func (m *mgr) delete(ctx context.Context) error {
	aliases, err := m.getAliases(ctx)
	if err != nil {
		return fmt.Errorf("get aliases: %w", err)
	}

	if len(aliases) == 0 {
		return nil
	}
	if len(aliases) > 1 {
		return fmt.Errorf("multiple aliases found, can't delete: %v", aliases)
	}

	alias := aliases[0]
	if err := m.api.DeleteAlias(ctx, alias.ZoneID, alias.UUID); err != nil {
		return fmt.Errorf("delete alias: %w", err)
	}
	return nil
}

// getAliases retrieves a list of aliases for the specified zone, name, and UUID or returns an error if unsuccessful.
func (m *mgr) getAliases(ctx context.Context) ([]sgcp.Alias, error) {
	// TODO: Use ageing cache
	method, url, code, data, err := m.api.GetAliases(ctx, m.alias.ZoneID, m.alias.Name, m.alias.UUID)
	if err != nil {
		return nil, err
	}
	if err := m.api.CheckStatusCode(method, url, code, http.StatusOK, http.StatusNotFound); err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, nil
	}
	var resp aliasListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode aliases: %w", err)
	}
	return resp.Aliases, nil
}

// create creates a new alias with the specified target and returns the created alias or an error if the operation fails.
func (m *mgr) create(ctx context.Context, target string) (*alias, error) {
	v, err := m.api.CreateAlias(ctx, m.alias.ZoneID, m.alias.Name, target)
	if err != nil {
		return nil, err
	}
	return toAlias(v), nil
}

// update modifies an existing alias with the specified parameters and returns the updated alias or an error if any occurs.
func (m *mgr) update(ctx context.Context, zoneID, aliasUUID, aliasName, target string) (*alias, error) {
	v, err := m.api.UpdateAlias(ctx, zoneID, aliasUUID, aliasName, target)
	if err != nil {
		return nil, err
	}
	return toAlias(v), nil
}

// toAlias converts a sgcp.Alias object to an alias object by mapping corresponding fields.
func toAlias(v *sgcp.Alias) *alias {
	return &alias{
		UUID:   v.UUID,
		Name:   v.Name,
		Target: v.Target,
		FQDN:   v.FQDN,
		ZoneID: v.ZoneID,
	}
}

// Equal compares two alias objects and returns true if they are equal, or false otherwise.
// It doesn't compare the FQDN field.
func (a *alias) Equal(b *alias) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.UUID == b.UUID &&
		a.Name == b.Name &&
		a.Target == b.Target &&
		a.ZoneID == b.ZoneID
}
