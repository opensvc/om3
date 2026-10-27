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
opensvc_pg_cgroup_memory_current_bytes{namespace="",path="",scope="opensvc"} 8192
opensvc_pg_cgroup_memory_current_bytes{namespace="",path="",scope="host"} 65536
opensvc_pg_cgroup_memory_current_bytes{namespace="",path="machine.slice",scope="slice"} 1024
opensvc_pg_cgroup_memory_current_bytes{namespace="prod",path="",scope="namespace"} 2048
`

func TestFilterMetricsTextKeepsTheNamespacesOfTheGrants(t *testing.T) {
	cases := []struct {
		name   string
		grants rbac.Grants
		kept   []string
		left   []string
	}{
		{"guest of prod", rbac.NewGrants("guest:prod"), []string{`path="prod/svc/web"`, `namespace="prod",path="",scope="namespace"`}, []string{`path="dev/svc/web"`, `path="svc/dns"`, `scope="opensvc"`, `scope="host"`, `scope="slice"`}},
		{"admin of dev", rbac.NewGrants("admin:dev"), []string{`path="dev/svc/web"`}, []string{`path="prod/svc/web"`, `path="svc/dns"`, `scope="opensvc"`, `scope="namespace"`}},
		{"guest of all namespaces", rbac.NewGrants("guest"), []string{`path="prod/svc/web"`, `path="dev/svc/web"`, `path="svc/dns"`, `scope="opensvc"`, `scope="host"`, `scope="slice"`}, nil},
		{"root", rbac.NewGrants("root"), []string{`path="prod/svc/web"`, `path="dev/svc/web"`, `path="svc/dns"`, `scope="opensvc"`, `scope="host"`, `scope="slice"`}, nil},
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
