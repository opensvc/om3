package restaskacme_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/restaskacme"
)

// certificatesAlerts returns the strategy alerts of the validation of the
// configuration of a svc, as "<level> <key>: <comment>".
func certificatesAlerts(t *testing.T, conf string) []string {
	t.Helper()
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "web"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf)), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		if !slices.Contains([]string{"missing strategy", "missing keyword", "conflicting strategies"}, m["kind"].(string)) {
			continue
		}
		l = append(l, m["level"].(string)+" "+m["key"].(string)+": "+m["comment"].(string))
	}
	return l
}

// An acme task renews the secs it names, the secs of the domains it reads,
// or both: one with neither has nothing to renew, and the domains are read
// with both domains_from and domains_to, which the run refuses alone. The
// validation says so when the configuration is written.
func TestValidateCertificatesStrategies(t *testing.T) {
	testhelper.Setup(t)
	require.NoError(t, capabilities.Scan(t.Context()))

	assert.Equal(t,
		[]string{"error task#1.: no certificates strategy is followed: set secs (secs), or domains_from or domains_to (domains)"},
		certificatesAlerts(t, "[task#1]\ntype = acme\n"), "nothing to renew")
	assert.Equal(t,
		[]string{"error task#1.domains_to: domains_to is not set, but the domains certificates strategy requires it"},
		certificatesAlerts(t, "[task#1]\ntype = acme\ndomains_from = vol1:/etc/domains\n"), "domains_from alone")
	assert.Equal(t,
		[]string{"error task#1.domains_from: domains_from is not set, but the domains certificates strategy requires it"},
		certificatesAlerts(t, "[task#1]\ntype = acme\ndomains_to = cert-*\n"), "domains_to alone")

	assert.Empty(t, certificatesAlerts(t, "[task#1]\ntype = acme\nsecs = cert1\n"), "secs named")
	assert.Empty(t, certificatesAlerts(t, "[task#1]\ntype = acme\ndomains_from = vol1:/etc/domains\ndomains_to = cert-*\n"), "domains read")
	assert.Empty(t, certificatesAlerts(t, "[task#1]\ntype = acme\nsecs = cert1\ndomains_from = vol1:/etc/domains\ndomains_to = cert-*\n"), "both, no conflict")
}
