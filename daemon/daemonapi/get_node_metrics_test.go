package daemonapi

import (
	"strings"
	"testing"

	"github.com/opensvc/om3/v3/daemon/rbac"
)

// A /metrics/pg page: object series labelled by namespace.
const pgMetricsPage = `# HELP opensvc_pg_cgroup_memory_current_bytes memory.current
# TYPE opensvc_pg_cgroup_memory_current_bytes gauge
opensvc_pg_cgroup_memory_current_bytes{namespace="prod",path="prod/svc/web"} 1024
opensvc_pg_cgroup_memory_current_bytes{namespace="dev",path="dev/svc/web"} 2048
opensvc_pg_cgroup_memory_current_bytes{namespace="root",path="svc/dns"} 4096
`

func TestFilterMetricsTextKeepsTheNamespacesOfTheGrants(t *testing.T) {
	cases := []struct {
		name   string
		grants rbac.Grants
		kept   []string
		left   []string
	}{
		{"guest of prod", rbac.NewGrants("guest:prod"), []string{`path="prod/svc/web"`}, []string{`path="dev/svc/web"`, `path="svc/dns"`}},
		{"admin of dev", rbac.NewGrants("admin:dev"), []string{`path="dev/svc/web"`}, []string{`path="prod/svc/web"`, `path="svc/dns"`}},
		{"guest of all namespaces", rbac.NewGrants("guest"), []string{`path="prod/svc/web"`, `path="dev/svc/web"`, `path="svc/dns"`}, nil},
		{"root", rbac.NewGrants("root"), []string{`path="prod/svc/web"`, `path="dev/svc/web"`, `path="svc/dns"`}, nil},
		{"no grant", rbac.NewGrants(), nil, []string{`path="prod/svc/web"`, `path="dev/svc/web"`, `path="svc/dns"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterMetricsText(pgMetricsPage, tc.grants)
			for _, s := range tc.kept {
				if !strings.Contains(got, s) {
					t.Errorf("expected %s kept, got:\n%s", s, got)
				}
			}
			for _, s := range tc.left {
				if strings.Contains(got, s) {
					t.Errorf("expected %s left out, got:\n%s", s, got)
				}
			}
		})
	}
}
