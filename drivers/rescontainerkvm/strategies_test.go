package rescontainerkvm_test

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
	"github.com/opensvc/om3/v3/util/hostname"

	_ "github.com/opensvc/om3/v3/drivers/rescontainerkvm"
)

// strategyAlerts returns the conflicting strategies alerts of the validation
// of the configuration of a svc, as "<level> <key>: <comment>".
func strategyAlerts(t *testing.T, conf string) []string {
	t.Helper()
	p := naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "vm"}
	o, err := object.NewSvc(p, object.WithConfigData([]byte("[DEFAULT]\nnodes = "+hostname.Hostname()+"\n"+conf)), object.WithVolatile(true))
	require.NoError(t, err)
	alerts, err := o.Config().Validate()
	require.NoError(t, err)
	l := make([]string, 0)
	for _, a := range alerts {
		m := a.Unstructured()
		if m["kind"] != "conflicting strategies" {
			continue
		}
		l = append(l, m["level"].(string)+" "+m["key"].(string)+": "+m["comment"].(string))
	}
	return l
}

// A guest made by a virtinst command and described by the keywords too, or
// installed from both a cloud image and an installer, is warned of, as its
// provision refuses it. One way alone is not, with the keywords describing
// the machine, its storage and its network.
func TestValidateConflictingInstallStrategies(t *testing.T) {
	env := testhelper.Setup(t)
	env.InstallFile("../../testdata/nodes_info.json", "var/nodes_info.json")
	env.InstallFile("../../testdata/cluster.conf", "etc/cluster.conf")
	_, err := object.SetClusterConfig()
	require.NoError(t, err)

	// The validation skips the sections of a driver the node can not use:
	// the node running the test may have no virsh.
	drvCap := driver.NewID(driver.GroupContainer, "kvm").Cap()
	capabilities.Register(func(context.Context) ([]string, error) { return []string{drvCap}, nil })
	require.NoError(t, capabilities.Scan(t.Context()))

	spec := "vcpus = 2\nmemory = 2g\ndisk = disk#1\n"

	assert.Equal(t,
		[]string{"warning container#1.vcpus: virtinst and vcpus are both set, but the virtinst install strategy uses virtinst and excludes vcpus"},
		strategyAlerts(t, "[container#1]\ntype = kvm\nvirtinst = virt-install --name vm1\nvcpus = 2\n"))

	assert.ElementsMatch(t, []string{
		"warning container#1.virtinst: image and virtinst are both set, but image is the image install strategy and virtinst the virtinst one",
		"warning container#1.virtinst_args: virtinst and virtinst_args are both set, but the virtinst install strategy uses virtinst and excludes virtinst_args",
	}, strategyAlerts(t, "[container#1]\ntype = kvm\nvirtinst = virt-install --name vm1\nimage = /srv/noble.qcow2\nvirtinst_args = --tpm none\n"))

	assert.ElementsMatch(t, []string{
		"warning container#1.cdrom: image and cdrom are both set, but image is the image install strategy and cdrom the cdrom one",
		"warning container#1.cdrom: cloud_init and cdrom are both set, but cloud_init is the image install strategy and cdrom the cdrom one",
	}, strategyAlerts(t, "[container#1]\ntype = kvm\n"+spec+"image = /srv/noble.qcow2\ncloud_init = from ./cfg/vm1 key user-data\ncdrom = /srv/debian.iso\n"))

	assert.Empty(t, strategyAlerts(t, "[container#1]\ntype = kvm\n"+spec+"image = /srv/noble.qcow2\ncloud_init = from ./cfg/vm1 key user-data\nnetwork = default\nbridge = br0\nosinfo = ubuntu24.04\nvirtinst_args = --tpm none\n"), "a cloud image")
	assert.Empty(t, strategyAlerts(t, "[container#1]\ntype = kvm\n"+spec+"cdrom = /srv/debian.iso\nnetwork = default\n"), "an installer")
	assert.Empty(t, strategyAlerts(t, "[container#1]\ntype = kvm\n"+spec), "a disk booted as it is")
	assert.Empty(t, strategyAlerts(t, "[container#1]\ntype = kvm\nvirtinst = virt-install --name vm1\nnetwork_filter = false\nvsock_port = 0\n"), "a command, and keywords it does not exclude")
}
