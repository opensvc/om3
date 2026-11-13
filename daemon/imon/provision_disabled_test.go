package imon

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
)

// A purge or an unprovision of an object a node evaluates unprovision=false
// on is refused upfront while that instance is provisioned, with the reason,
// and goes through once the sysadmin freed it and declared it so.
func TestAnOrchestrationTheConfigurationDisablesIsRefusedUpfront(t *testing.T) {
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "protected"}
	set := func(node string, unprovisionDisabled bool, prov provisioned.T) {
		instance.ConfigData.Set(p, node, &instance.Config{ActorConfig: &instance.ActorConfig{UnprovisionDisabled: unprovisionDisabled, ProvisionDisabled: unprovisionDisabled}})
		instance.StatusData.Set(p, node, &instance.Status{Provisioned: prov})
	}
	t.Cleanup(func() {
		for _, node := range []string{"n1", "n2"} {
			instance.ConfigData.Unset(p, node)
			instance.StatusData.Unset(p, node)
		}
	})
	m := &Manager{path: p}

	set("n1", true, provisioned.True)
	set("n2", false, provisioned.True)
	reason := m.provisionDisabledRefusal(instance.MonitorGlobalExpectPurged)
	assert.Contains(t, reason, "unprovision is disabled (unprovision=false) on n1")
	assert.NotContains(t, reason, "n2")
	assert.Contains(t, m.provisionDisabledRefusal(instance.MonitorGlobalExpectUnprovisioned), "on n1")
	assert.Empty(t, m.provisionDisabledRefusal(instance.MonitorGlobalExpectProvisioned), "provisioned already")

	set("n1", true, provisioned.False)
	assert.Empty(t, m.provisionDisabledRefusal(instance.MonitorGlobalExpectPurged), "freed by the sysadmin")
	assert.Contains(t, m.provisionDisabledRefusal(instance.MonitorGlobalExpectProvisioned), "provision is disabled (provision=false) on n1")
}
