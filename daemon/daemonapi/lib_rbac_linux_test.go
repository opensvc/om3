//go:build linux

package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/rbac"

	// The policy is about driver keywords, which exist only once the drivers
	// are registered. Without this the configuration below evaluates to
	// nothing and the test passes for the wrong reason.
	_ "github.com/opensvc/om3/v3/core/driverdb"
)

// rbacOf runs the policy over a whole configuration, the way the api does on
// every write, and returns the first refusal.
func rbacOf(t *testing.T, config string) error {
	t.Helper()
	p, err := naming.ParsePath("test/svc/foo")
	require.NoError(t, err)
	o, err := object.New(p, object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)
	return configRbacKeys(rbac.Grants{}, o.(object.Configurer).Config())
}

// TestConfigRbacReadsTheValueTheConfigurationSpells is the regression test of a
// keyword converted to a list.
//
// Evaluating such a keyword returns a []string, which prints inside brackets.
// The policy was handed "[_/etc:/etc:ro]" where the configuration says
// "_/etc:/etc:ro", so the mount did not begin with the underscore that marks a
// host path, and a rule meant to refuse exactly this let it through.
func TestConfigRbacReadsTheValueTheConfigurationSpells(t *testing.T) {
	assert.Error(t, rbacOf(t, `
[container#0]
type = docker
image = busybox
volume_mounts = _/etc:/etc:ro
`), "a single host path mount is the whole value, so it was the whole bracketed string")

	assert.Error(t, rbacOf(t, `
[container#0]
type = docker
image = busybox
volume_mounts = _/etc:/etc:ro /vol/a:/a
`))

	assert.Error(t, rbacOf(t, `
[volume#0]
name = v1
install = /etc/passwd source /etc/shadow
`), "a server-local source is named in the middle of a shlex list")

	// The same keywords with nothing to refuse stay writable.
	assert.NoError(t, rbacOf(t, `
[container#0]
type = docker
image = busybox
volume_mounts = /vol/data:/data:rw /vol/etc:/etc:ro
`))

	assert.NoError(t, rbacOf(t, `
[volume#0]
name = v1
install = /etc/nginx.conf source https://example.com/nginx.conf
`))
}

// TestConfigRbacAllowsEveryValueOfAList pins that a list keyword is allowed
// when every value it names is, and refused as soon as one is not.
func TestConfigRbacAllowsEveryValueOfAList(t *testing.T) {
	assert.NoError(t, rbacOf(t, "[DEFAULT]\nmonitor_action = switch\n"))
	assert.NoError(t, rbacOf(t, "[DEFAULT]\nmonitor_action = switch freezestop\n"))
	assert.Error(t, rbacOf(t, "[DEFAULT]\nmonitor_action = reboot\n"))
	assert.Error(t, rbacOf(t, "[DEFAULT]\nmonitor_action = switch reboot\n"),
		"one unsafe action among safe ones is still an unsafe action")
}

// TestConfigRbacResolvesReferences pins that the policy reads what a keyword
// resolves to, not the reference that produced it. A value a user may not set
// is one they may not set through a section the policy does not gate.
func TestConfigRbacResolvesReferences(t *testing.T) {
	assert.Error(t, rbacOf(t, `
[env]
mounts = _/etc:/etc:ro
[container#0]
type = docker
image = busybox
volume_mounts = {env.mounts}
`))

	assert.Error(t, rbacOf(t, `
[env]
typ = kvm
[container#0]
type = {env.typ}
`))

	assert.Error(t, rbacOf(t, `
[env]
action = reboot
[DEFAULT]
monitor_action = {env.action}
`))

	assert.NoError(t, rbacOf(t, `
[env]
typ = docker
[container#0]
type = {env.typ}
image = busybox
`))
}

// TestConfigRbacIPDrawnFromAClusterNetwork is the ip.netns rule seen from the
// api: the setup om allocates the address in is writable, the ones where the
// user names it are not.
func TestConfigRbacIPDrawnFromAClusterNetwork(t *testing.T) {
	assert.NoError(t, rbacOf(t, `
[ip#0]
type = netns
network = default
netns = container#0
`))

	assert.Error(t, rbacOf(t, `
[ip#0]
type = netns
network = default
netns = container#0
name = 10.0.0.5
`))

	assert.Error(t, rbacOf(t, `
[env]
addr = 10.0.0.5
[ip#0]
type = netns
network = default
name = {env.addr}
`))

	assert.Error(t, rbacOf(t, `
[ip#0]
type = netns
netns = container#0
`), "a netns resource drawing from no network has an address of its own")

	assert.Error(t, rbacOf(t, `
[ip#0]
type = host
network = default
`))

	// An ip.cni resource is writable as it was before the ip group gained a
	// default, including the keywords every section carries.
	assert.NoError(t, rbacOf(t, `
[ip#0]
type = cni
network = default
netns = container#0
expose = 8080/tcp
comment = a comment
optional = true
tags = t1 t2
pg_cpus = 0-1
`))
}
