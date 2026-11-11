package object_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/capabilities"

	_ "github.com/opensvc/om3/v3/drivers/resappforking"
	_ "github.com/opensvc/om3/v3/drivers/rescontainerpodman"
	_ "github.com/opensvc/om3/v3/drivers/resfsflag"
	_ "github.com/opensvc/om3/v3/drivers/resipnetns"
	_ "github.com/opensvc/om3/v3/drivers/ressyncstore"
	_ "github.com/opensvc/om3/v3/drivers/restaskhost"
)

// referenceAlert is what the validation says of a line, as its kind, key,
// level and comment, the fields the designer reads.
type referenceAlert struct {
	Kind, Key, Level, Comment string
}

// validateReferences returns the alerts the validation of a configuration
// made of the sections every case has and lines says of the values naming
// resources, and of the values a keyword does not take.
func validateReferences(t *testing.T, lines string) []referenceAlert {
	t.Helper()
	return validateReferencesWithDefaults(t, "", lines)
}

// validateReferencesWithDefaults is validateReferences with defaults written
// in the DEFAULT section.
func validateReferencesWithDefaults(t *testing.T, defaults, lines string) []referenceAlert {
	t.Helper()
	testhelper.Setup(t)
	caps := []string{
		driver.NewID(driver.GroupApp, "forking").Cap(),
		driver.NewID(driver.GroupContainer, "podman").Cap(),
		driver.NewID(driver.GroupFS, "flag").Cap(),
		driver.NewID(driver.GroupIP, "netns").Cap(),
		driver.NewID(driver.GroupSync, "store").Cap(),
		driver.NewID(driver.GroupSync, "symsnapvx").Cap(),
		driver.NewID(driver.GroupTask, "host").Cap(),
	}
	capabilities.Register(func(context.Context) ([]string, error) { return caps, nil })
	require.NoError(t, capabilities.Scan(t.Context()))
	p, err := naming.ParsePath("test/svc/web")
	require.NoError(t, err)
	o, err := object.NewSvc(p, object.WithVolatile(true), object.WithConfigData([]byte(`
[DEFAULT]
nodes = *
`+defaults+`

[container#1]
type = podman
image = ghcr.io/opensvc/pause

[fs#1]
type = flag

[app#1]
type = forking
start = /bin/true
`+lines+"\n")))
	require.NoError(t, err)
	alerts, err := o.ValidateConfig(context.Background())
	require.NoError(t, err)
	var l []referenceAlert
	for _, alert := range alerts {
		switch s := alert.Kind.String(); s {
		case "unknown resource", "unsupported resource group", "unsupported value":
			l = append(l, referenceAlert{Kind: s, Key: alert.Key.String(), Level: alert.Level.String(), Comment: alert.Comment})
		}
	}
	return l
}

// A keyword whose value names resources of the object has the validation say
// of a resource named that the configuration has no section of, a warning as
// a configuration may be written in steps, and of a resource of a group the
// keyword does not take, an error. A value naming no resource, a candidate
// or a value the engine takes as it is, is not judged as a resource.
func TestValidateReferences(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines string
		want  []referenceAlert
	}{
		{
			name:  "a resource named exists",
			lines: "[container#2]\ntype = podman\nimage = ghcr.io/opensvc/pause\nnetns = container#1\npidns = container#1\nipcns = container#1",
		},
		{
			name:  "unknown section warns",
			lines: "[container#2]\ntype = podman\nimage = ghcr.io/opensvc/pause\nnetns = container#9",
			want: []referenceAlert{
				{Kind: "unknown resource", Key: "container#2.netns", Level: "warning", Comment: "container#9 names no section of the configuration"},
			},
		},
		{
			name:  "wrong group errors",
			lines: "[ip#1]\ntype = netns\nnetns = fs#1",
			want: []referenceAlert{
				{Kind: "unsupported resource group", Key: "ip#1.netns", Level: "error", Comment: "fs#1 is no container resource"},
			},
		},
		{
			name:  "wrong group of many errors",
			lines: "[app#2]\ntype = forking\nstart = /bin/true\nnetns = fs#1",
			want: []referenceAlert{
				{Kind: "unsupported resource group", Key: "app#2.netns", Level: "error", Comment: "fs#1 is no ip, container or app resource"},
			},
		},
		{
			name:  "candidates accepted",
			lines: "[container#2]\ntype = podman\nimage = ghcr.io/opensvc/pause\nnetns = host\npidns = host\nipcns = shareable",
		},
		{
			name:  "raw engine value on netns errors",
			lines: "[container#2]\ntype = podman\nimage = ghcr.io/opensvc/pause\nnetns = container:x",
			want: []referenceAlert{
				{Kind: "unsupported value", Key: "container#2.netns", Level: "error", Comment: "container:x"},
			},
		},
		{
			name:  "requires with states",
			lines: "[app#2]\ntype = forking\nstart = /bin/true\nstart_requires = fs#1(up,stdby up) ip#9(down, stdby down) container#1",
			want: []referenceAlert{
				{Kind: "unknown resource", Key: "app#2.start_requires", Level: "warning", Comment: "ip#9 names no section of the configuration"},
			},
		},
		{
			name:  "rids list",
			lines: "[sync#1]\ntype = store\nstore = backup\nrids = fs#1 container#1 fs#9",
			want: []referenceAlert{
				{Kind: "unsupported resource group", Key: "sync#1.rids", Level: "error", Comment: "container#1 is no fs or volume resource"},
				{Kind: "unknown resource", Key: "sync#1.rids", Level: "warning", Comment: "fs#9 names no section of the configuration"},
			},
		},
		{
			name:  "podman userns raw mode",
			lines: "[container#2]\ntype = podman\nimage = ghcr.io/opensvc/pause\nuserns = keep-id:uid=1000",
		},
		{
			name:  "podman userns container",
			lines: "[container#2]\ntype = podman\nimage = ghcr.io/opensvc/pause\nuserns = fs#1",
			want: []referenceAlert{
				{Kind: "unsupported resource group", Key: "container#2.userns", Level: "error", Comment: "fs#1 is no container resource"},
			},
		},
		{
			// The devices of a snapshot are the sub-devices of the
			// resources named, which a fs and a container list too.
			name:  "symsnapvx devices from a fs and a container",
			lines: "[sync#2]\ntype = symsnapvx\nsymid = 000197900123\ndevs_from = fs#1 container#1 app#1",
			want: []referenceAlert{
				{Kind: "unsupported resource group", Key: "sync#2.devs_from", Level: "error", Comment: "app#1 is no disk, fs, volume or container resource"},
			},
		},
		{
			name:  "task in a container",
			lines: "[task#1]\ntype = host\ncommand = /bin/true\ncontainer = container#1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.ElementsMatch(t, tc.want, validateReferences(t, tc.lines))
		})
	}
}

// A value written in the DEFAULT section is inherited by the resources not
// setting the keyword, of any driver: an app netns takes an ip, a container
// netns does not. The groups are not judged there, the resources named are.
func TestValidateReferencesInDefault(t *testing.T) {
	ip := "[ip#1]\ntype = netns\nnetns = container#1"
	assert.Empty(t, validateReferencesWithDefaults(t, "netns = ip#1", ip))
	assert.Equal(t, []referenceAlert{
		{Kind: "unknown resource", Key: "netns", Level: "warning", Comment: "container#9 names no section of the configuration"},
	}, validateReferencesWithDefaults(t, "netns = container#9", ip))
}

// The install of a task refuses a resource of another group than fs and
// volume by its own check, which the references do not say again.
func TestValidateReferencesInstallSaidOnce(t *testing.T) {
	alerts := validateReferences(t, "[task#1]\ntype = host\ninstall = app#1")
	require.Len(t, alerts, 1)
	assert.Equal(t, "unsupported value", alerts[0].Kind)
	assert.Equal(t, "install app#1: app#1 is not a fs or volume resource", alerts[0].Comment)
}
