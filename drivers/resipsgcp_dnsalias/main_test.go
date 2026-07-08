package resipsgcp_dnsalias

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sgcp"
)

type mockTokenGetter struct {
	token string
}

func (m *mockTokenGetter) Get(ctx context.Context, scope ...string) (string, error) {
	return m.token, nil
}

func newTestDNSAPI(t *testing.T, handler func(w http.ResponseWriter, r *http.Request) (int, interface{})) (*sgcp.DNSAPI, *httptest.Server) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statusCode, resp := handler(w, r)
		if resp != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Errorf("failed to encode response: %v", err)
			}
		} else {
			w.WriteHeader(statusCode)
		}
	}))

	cfg := &sgcp.Config{
		DNS: sgcp.DNSConfig{
			BaseURL: server.URL,
			Path: struct {
				Alias string `yaml:"alias"`
				Zone  string `yaml:"zone"`
			}{
				Alias: "/aliases",
				Zone:  "/zones",
			},
		},
		Auth: sgcp.AuthConfig{
			Scopes: map[string][]string{
				"dns_read":  {"dns:read_dns_records"},
				"dns_write": {"dns:admin_dns_records"},
			},
		},
	}
	client := server.Client()
	logger := plog.NewDefaultLogger()
	tk := &mockTokenGetter{token: "fake-token"}

	api := sgcp.NewDNSAPI(cfg, client, logger, tk)
	return api, server
}

func TestManagerCreateOrUpdateCreatesAliasWhenMissing(t *testing.T) {
	var requests int
	api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		requests++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aliases":
			return http.StatusOK, map[string]interface{}{"aliases": []interface{}{}}
		case r.Method == http.MethodPost && r.URL.Path == "/zones/zone-1/aliases":
			return http.StatusCreated, map[string]interface{}{
				"id":      "alias-1",
				"name":    "svc1",
				"target":  "node1",
				"fqdn":    "svc1.example.org",
				"zone_id": "zone-1",
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return http.StatusNotFound, nil
		}
	})
	defer server.Close()

	mgr := &aliasManager{alias: alias{ZoneID: "zone-1", Name: "svc1", Target: "node1"}, api: api}

	if err := mgr.createOrUpdate(context.Background(), "node1"); err != nil {
		t.Fatalf("createOrUpdate returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
	if mgr.alias.UUID != "alias-1" {
		t.Fatalf("expected alias uuid alias-1, got %s", mgr.alias.UUID)
	}
}

func TestManagerCreateOrUpdateUpdatesExistingAlias(t *testing.T) {
	var requests int
	api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		requests++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aliases":
			return http.StatusOK, map[string]interface{}{
				"aliases": []interface{}{
					map[string]interface{}{
						"id":      "alias-1",
						"name":    "svc1",
						"target":  "old-node",
						"fqdn":    "svc1.example.org",
						"zone_id": "zone-1",
					},
				},
			}
		case r.Method == http.MethodPut && r.URL.Path == "/zones/zone-1/aliases/alias-1":
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("invalid payload: %v", err)
				return http.StatusBadRequest, nil
			}
			if payload["name"] != "svc1" || payload["target"] != "node1" {
				t.Errorf("unexpected payload: %v", payload)
				return http.StatusBadRequest, nil
			}
			return http.StatusOK, map[string]interface{}{
				"id":      "alias-1",
				"name":    "svc1",
				"target":  "node1",
				"fqdn":    "svc1.example.org",
				"zone_id": "zone-1",
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return http.StatusNotFound, nil
		}
	})
	defer server.Close()

	mgr := &aliasManager{alias: alias{ZoneID: "zone-1", Name: "svc1", Target: "node1"}, api: api}

	if err := mgr.createOrUpdate(context.Background(), "node1"); err != nil {
		t.Fatalf("createOrUpdate returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
	if mgr.alias.Target != "node1" {
		t.Fatalf("expected target node1, got %s", mgr.alias.Target)
	}
}

func TestManagerCreateOrUpdateMultipleAliasesError(t *testing.T) {
	api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		if r.Method == http.MethodGet && r.URL.Path == "/aliases" {
			return http.StatusOK, map[string]interface{}{
				"aliases": []interface{}{
					map[string]interface{}{"id": "a1", "name": "svc1", "target": "node1", "zone_id": "zone-1"},
					map[string]interface{}{"id": "a2", "name": "svc1", "target": "node2", "zone_id": "zone-1"},
				},
			}
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		return http.StatusNotFound, nil
	})
	defer server.Close()

	mgr := &aliasManager{alias: alias{ZoneID: "zone-1", Name: "svc1"}, api: api}

	err := mgr.createOrUpdate(context.Background(), "node1")
	if err == nil {
		t.Fatal("expected error for multiple aliases, got nil")
	}
	if !strings.Contains(err.Error(), "multiple aliases") {
		t.Errorf("expected error containing 'multiple aliases', got %v", err)
	}
}

func TestManagerCreateOrUpdateWithUUIDNotFoundError(t *testing.T) {
	api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		if r.Method == http.MethodGet && r.URL.Path == "/aliases" {
			return http.StatusOK, map[string]interface{}{"aliases": []interface{}{}}
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		return http.StatusNotFound, nil
	})
	defer server.Close()

	mgr := &aliasManager{alias: alias{ZoneID: "zone-1", UUID: "uuid-1", Name: "svc1"}, api: api}

	err := mgr.createOrUpdate(context.Background(), "node1")
	if err == nil {
		t.Fatal("expected error when UUID provided but alias not found")
	}
	if !strings.Contains(err.Error(), "no such alias") {
		t.Errorf("expected error containing 'no such alias', got %v", err)
	}
}

func TestManagerDeleteDeletesAlias(t *testing.T) {
	var requests int
	api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		requests++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aliases":
			return http.StatusOK, map[string]interface{}{
				"aliases": []interface{}{
					map[string]interface{}{"id": "alias-1", "name": "svc1", "target": "node1", "zone_id": "zone-1"},
				},
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/zones/zone-1/aliases/alias-1":
			return http.StatusNoContent, nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return http.StatusNotFound, nil
		}
	})
	defer server.Close()

	mgr := &aliasManager{alias: alias{ZoneID: "zone-1", Name: "svc1"}, api: api}

	if err := mgr.delete(context.Background()); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected 2 requests, got %d", requests)
	}
}

func TestManagerDeleteWhenAliasMissingDoesNothing(t *testing.T) {
	var requests int
	api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
		requests++
		if r.Method == http.MethodGet && r.URL.Path == "/aliases" {
			return http.StatusOK, map[string]interface{}{"aliases": []interface{}{}}
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		return http.StatusNotFound, nil
	})
	defer server.Close()

	mgr := &aliasManager{alias: alias{ZoneID: "zone-1", Name: "svc1"}, api: api}

	if err := mgr.delete(context.Background()); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected 1 request (only GET), got %d", requests)
	}
}

func TestTStatus(t *testing.T) {
	tests := []struct {
		name         string
		aliasResp    []interface{}
		configUUID   string
		configName   string
		configTarget string
		wantStatus   status.T
	}{
		{
			name:         "alias not found => Down",
			aliasResp:    []interface{}{},
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Down,
		},
		{
			name: "alias found with target none. => Down",
			aliasResp: []interface{}{
				map[string]interface{}{"id": "a1", "name": "svc1", "target": "none.", "fqdn": "svc1.example.org", "zone_id": "z1"},
			},
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Down,
		},
		{
			name: "alias target mismatch => Down",
			aliasResp: []interface{}{
				map[string]interface{}{"id": "a1", "name": "svc1", "target": "other", "fqdn": "svc1.example.org", "zone_id": "z1"},
			},
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Down,
		},
		{
			name: "name mismatch => Warn",
			aliasResp: []interface{}{
				map[string]interface{}{"id": "a1", "name": "wrong", "target": "node1", "fqdn": "svc1.example.org", "zone_id": "z1"},
			},
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Warn,
		},
		{
			name: "UUID mismatch => Warn",
			aliasResp: []interface{}{
				map[string]interface{}{"id": "wrong-uuid", "name": "svc1", "target": "node1", "fqdn": "svc1.example.org", "zone_id": "z1"},
			},
			configUUID:   "expected-uuid",
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Warn,
		},
		{
			name: "multiple aliases => Warn",
			aliasResp: []interface{}{
				map[string]interface{}{"id": "a1", "name": "svc1", "target": "node1", "zone_id": "z1"},
				map[string]interface{}{"id": "a2", "name": "svc1", "target": "node1", "zone_id": "z1"},
			},
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Warn,
		},
		{
			name: "perfect match => Up",
			aliasResp: []interface{}{
				map[string]interface{}{"id": "a1", "name": "svc1", "target": "node1", "fqdn": "svc1.example.org", "zone_id": "z1"},
			},
			configUUID:   "a1",
			configName:   "svc1",
			configTarget: "node1",
			wantStatus:   status.Up,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
				if r.Method == http.MethodGet && r.URL.Path == "/aliases" {
					return http.StatusOK, map[string]interface{}{"aliases": tt.aliasResp}
				}
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return http.StatusNotFound, nil
			})
			defer server.Close()

			tRes := &T{
				UUID:       tt.configUUID,
				Name:       tt.configName,
				Target:     tt.configTarget,
				ZoneID:     "z1",
				Endpoint:   server.URL,
				api:        api,
				configured: true,
			}

			got := tRes.Status(context.Background())
			if got != tt.wantStatus {
				t.Errorf("Status() = %v, want %v", got, tt.wantStatus)
			}
		})
	}
}

func TestTStartStop(t *testing.T) {
	t.Run("Start creates alias", func(t *testing.T) {
		api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/aliases":
				return http.StatusOK, map[string]interface{}{"aliases": []interface{}{}}
			case r.Method == http.MethodPost && r.URL.Path == "/zones/zone-1/aliases":
				return http.StatusCreated, map[string]interface{}{
					"id": "a1", "name": "svc1", "target": "node1", "fqdn": "svc1.example.org", "zone_id": "zone-1",
				}
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return http.StatusNotFound, nil
			}
		})
		defer server.Close()

		tRes := &T{
			Name:       "svc1",
			Target:     "node1",
			ZoneID:     "zone-1",
			Endpoint:   server.URL,
			api:        api,
			configured: true,
		}

		if err := tRes.Start(context.Background()); err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	})

	t.Run("Stop with UUID sets target to none.", func(t *testing.T) {
		api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/aliases":
				return http.StatusOK, map[string]interface{}{
					"aliases": []interface{}{
						map[string]interface{}{"id": "a1", "name": "svc1", "target": "node1", "zone_id": "zone-1"},
					},
				}
			case r.Method == http.MethodPut && r.URL.Path == "/zones/zone-1/aliases/a1":
				var payload map[string]interface{}
				json.NewDecoder(r.Body).Decode(&payload)
				if payload["target"] != "none." {
					t.Errorf("expected target 'none.', got %v", payload["target"])
				}
				return http.StatusOK, map[string]interface{}{
					"id": "a1", "name": "svc1", "target": "none.", "fqdn": "svc1.example.org", "zone_id": "zone-1",
				}
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return http.StatusNotFound, nil
			}
		})
		defer server.Close()

		tRes := &T{
			UUID:       "a1",
			Name:       "svc1",
			Target:     "node1",
			ZoneID:     "zone-1",
			Endpoint:   server.URL,
			api:        api,
			configured: true,
		}

		if err := tRes.Stop(context.Background()); err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	})

	t.Run("Stop without UUID deletes alias", func(t *testing.T) {
		api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/aliases":
				return http.StatusOK, map[string]interface{}{
					"aliases": []interface{}{
						map[string]interface{}{"id": "a1", "name": "svc1", "target": "node1", "zone_id": "zone-1"},
					},
				}
			case r.Method == http.MethodDelete && r.URL.Path == "/zones/zone-1/aliases/a1":
				return http.StatusNoContent, nil
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
				return http.StatusNotFound, nil
			}
		})
		defer server.Close()

		tRes := &T{
			Name:       "svc1",
			Target:     "node1",
			ZoneID:     "zone-1",
			Endpoint:   server.URL,
			api:        api,
			configured: true,
		}

		if err := tRes.Stop(context.Background()); err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	})
}

func TestAliasAPIHTTPErrors(t *testing.T) {
	t.Run("GetAliases returns error on non-200", func(t *testing.T) {
		api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
			return http.StatusInternalServerError, nil
		})
		defer server.Close()

		_, _, code, _, err := api.GetAliases(context.Background(), "z1", "", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", code)
		}
	})

	t.Run("CreateAlias returns error on non-201", func(t *testing.T) {
		api, server := newTestDNSAPI(t, func(w http.ResponseWriter, r *http.Request) (int, interface{}) {
			return http.StatusBadRequest, map[string]string{"error": "bad request"}
		})
		defer server.Close()

		_, _, code, _, err := api.CreateAlias(context.Background(), "z1", "svc1", "node1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", code)
		}
	})
}
