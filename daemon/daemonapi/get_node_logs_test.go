package daemonapi

import (
	"testing"

	"github.com/opensvc/om3/v3/daemon/rbac"
)

func TestReadableLogEntriesKeepTheNamespacesOfTheGrants(t *testing.T) {
	entries := []map[string]any{
		{"OBJ_PATH": "prod/svc/web"},
		{"OBJ_PATH": "dev/svc/web"},
		{"OBJ_PATH": "svc/dns"},
		// The configuration of the root namespace.
		{"OBJ_PATH": "/"},
		{"MESSAGE": "daemon: an entry of no object"},
	}
	cases := []struct {
		name   string
		grants rbac.Grants
		kept   []bool
	}{
		{"guest of prod", rbac.NewGrants("guest:prod"), []bool{true, false, false, false, false}},
		{"operator of root", rbac.NewGrants("operator:root"), []bool{false, false, true, true, false}},
		{"guest of all namespaces", rbac.NewGrants("guest"), []bool{true, true, true, true, false}},
		{"no grant", rbac.NewGrants(), []bool{false, false, false, false, false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			readable := readableLogEntries(tc.grants)
			if readable == nil {
				t.Fatalf("expected the entries sorted out")
			}
			for i, m := range entries {
				if got := readable(m); got != tc.kept[i] {
					t.Errorf("%v: expected kept %v, got %v", m, tc.kept[i], got)
				}
			}
		})
	}
	if readableLogEntries(rbac.NewGrants("root")) != nil {
		t.Errorf("expected root to read every entry")
	}
}

func TestHasObjectAccess(t *testing.T) {
	if hasObjectAccess(rbac.NewGrants()) {
		t.Errorf("expected no access with no grant")
	}
	if !hasObjectAccess(rbac.NewGrants("guest:prod")) {
		t.Errorf("expected access with a namespace grant")
	}
	if !hasObjectAccess(rbac.NewGrants("operator")) {
		t.Errorf("expected access with a cluster-wide grant")
	}
}
