//go:build linux

package rescontainerkvm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

func TestWithVsockDevice(t *testing.T) {
	args := []string{"/usr/bin/virt-install", "--name", "vm1"}
	got := withVsockDevice(args)
	assert.Equal(t, []string{"/usr/bin/virt-install", "--name", "vm1", "--vsock", "cid.auto=yes"}, got)
	assert.Len(t, args, 3, "the keyword value is left as it is")

	assert.True(t, isVirtInstall(args))
	assert.True(t, isVirtInstall([]string{"virt-install"}))
	assert.False(t, isVirtInstall([]string{"/opt/create-guest.sh"}), "a custom script")
	assert.False(t, isVirtInstall(nil))
}

func TestVirtInstWithVsockLeavesTheCommand(t *testing.T) {
	for name, r := range map[string]*T{
		"no port":       {VsockPort: 0, VirtInst: []string{"virt-install", "--name", "vm1"}},
		"custom script": {VsockPort: 1215, VirtInst: []string{"/opt/create-guest.sh"}},
	} {
		assert.Equal(t, r.VirtInst, r.virtInstWithVsock(context.Background(), r.VirtInst), name)
	}
}

// The definition a node syncs to a hypervisor with no vsock support starts
// there from a copy without its device, otherwise the same.
func TestDropVsockDevice(t *testing.T) {
	const head = "<domain>\n  <devices>\n    <interface type='bridge'/>\n"
	const tail = "    <memballoon model='virtio'/>\n  </devices>\n</domain>\n"
	for name, device := range map[string]string{
		"allocated": "    <vsock model='virtio'>\n      <cid auto='yes'/>\n    </vsock>\n",
		"with pci":  "    <vsock model='virtio'>\n      <cid auto='yes' address='3'/>\n      <address type='pci' bus='0x07'/>\n    </vsock>\n",
		"empty":     "    <vsock model='virtio'/>\n",
	} {
		got, ok := dropVsockDevice([]byte(head + device + tail))
		assert.True(t, ok, name)
		assert.Equal(t, head+tail, string(got), name)
	}
	got, ok := dropVsockDevice([]byte(head + tail))
	assert.False(t, ok)
	assert.Equal(t, head+tail, string(got))
}

// A hypervisor with no vsock support is not asked for a device it can not
// give a guest.
func TestVirtInstWithVsockOnAHostWithNoVsock(t *testing.T) {
	saved := hostVsockDevice
	hostVsockDevice = filepath.Join(t.TempDir(), "none")
	defer func() { hostVsockDevice = saved }()
	assert.False(t, hasHostVsock())
	r := &T{VsockPort: 1215, VirtInst: []string{"virt-install", "--name", "vm1"}}
	r.SetLoggerForTest(plog.NewDefaultLogger())
	assert.Equal(t, r.VirtInst, r.virtInstWithVsock(context.Background(), r.VirtInst))
}

func TestVsockCID(t *testing.T) {
	for s, want := range map[string]int{
		`<vsock model='virtio'><cid auto='yes' address='3'/></vsock>`: 3,
		`<vsock model="virtio"><cid auto="no" address="42"/></vsock>`: 42,
		`<cid address='7' auto='no'/>`:                                7,
	} {
		got, ok := vsockCID([]byte(s))
		assert.True(t, ok, s)
		assert.Equal(t, want, got, s)
	}
	for _, s := range []string{
		`<domain></domain>`,
		`<vsock model='virtio'><cid auto='yes'/></vsock>`,
	} {
		_, ok := vsockCID([]byte(s))
		assert.False(t, ok, s)
	}
}

// A virsh create of the domain leaves its definition, and what the nodes sync.
func TestDefinitionWithoutVsock(t *testing.T) {
	saved := configDir
	configDir = t.TempDir()
	defer func() { configDir = saved }()
	r := &T{Name: "vm1"}

	path, err := r.definitionWithoutVsock()
	assert.Error(t, err, "no configuration file")
	assert.Empty(t, path)

	withDevice := "<domain>\n  <vsock model='virtio'><cid auto='yes'/></vsock>\n</domain>\n"
	require.NoError(t, os.WriteFile(r.configFile(), []byte(withDevice), 0o644))
	path, err = r.definitionWithoutVsock()
	require.NoError(t, err)
	defer func() { _ = os.Remove(path) }()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "<domain>\n</domain>\n", string(b))
	b, err = os.ReadFile(r.configFile())
	require.NoError(t, err)
	assert.Equal(t, withDevice, string(b), "the definition is left as it is")

	require.NoError(t, os.WriteFile(r.configFile(), []byte("<domain>\n</domain>\n"), 0o644))
	path, err = r.definitionWithoutVsock()
	require.NoError(t, err)
	assert.Empty(t, path, "no device to drop")
}

func TestVMSpecArgs(t *testing.T) {
	s := vmSpec{
		name:     "vm1",
		vcpus:    2,
		memoryMB: 2048,
		osFlag:   "--osinfo",
		osValue:  "ubuntu24.04",
		disks:    []string{"/dev/zvol/tank/vm1", "/dev/sdb"},
		bridges:  []string{"br-prd"},
		userData: "/tmp/ud",
		extra:    []string{"--boot", "uefi"},
	}
	assert.Equal(t, []string{
		"virt-install", "--name", "vm1", "--memory", "2048", "--vcpus", "2", "--osinfo", "ubuntu24.04", "--import",
		"--disk", "/dev/zvol/tank/vm1,format=raw,cache=none", "--disk", "/dev/sdb,format=raw,cache=none",
		"--network", "bridge=br-prd,model=virtio",
		"--cloud-init", "user-data=/tmp/ud",
		"--graphics", "vnc,listen=socket",
		"--noautoconsole",
		"--boot", "uefi",
	}, s.args())
}

// A virt-install older than the --osinfo name takes --os-variant, and a guest
// naming no os does not stop a virt-install requiring one.
func TestOSOption(t *testing.T) {
	for _, tc := range []struct {
		name, help, osinfo, flag, value string
	}{
		{"recent, detected", "  --osinfo OSINFO", "", "--osinfo", "detect=on,require=off"},
		{"recent, named", "  --osinfo OSINFO", "rhel9.0", "--osinfo", "rhel9.0"},
		{"older, generic", "  --os-variant", "", "--os-variant", "generic"},
		{"older, named", "  --os-variant", "rhel9.0", "--os-variant", "rhel9.0"},
	} {
		flag, value := osOption([]byte(tc.help), tc.osinfo)
		assert.Equal(t, tc.flag, flag, tc.name)
		assert.Equal(t, tc.value, value, tc.name)
	}
}

func TestSpecError(t *testing.T) {
	size := int64(1 << 30)
	assert.NoError(t, (&T{}).specError(), "a domain defined by a sysadmin")
	assert.NoError(t, (&T{VirtInst: []string{"virt-install"}}).specError(), "virtinst alone")
	assert.NoError(t, (&T{VCPUs: 2, Memory: &size, Disk: []string{"disk#1"}}).specError())
	assert.Error(t, (&T{VCPUs: 2, Memory: &size, Disk: []string{"disk#1"}, VirtInst: []string{"x"}}).specError(), "virtinst says all of it")
	assert.ErrorContains(t, (&T{VCPUs: 2}).specError(), "memory, disk")
	assert.ErrorContains(t, (&T{Image: "/a"}).specError(), "vcpus, memory, disk")
	small := int64(512)
	assert.ErrorContains(t, (&T{VCPUs: 2, Memory: &small, Disk: []string{"disk#1"}}).specError(), "memory")
}

func TestParseImageInfo(t *testing.T) {
	format, err := parseImageInfo([]byte(`{"format": "qcow2", "virtual-size": 1024}`))
	assert.NoError(t, err)
	assert.Equal(t, "qcow2", format)
	_, err = parseImageInfo([]byte(`{"format": "qcow2", "backing-filename": "/etc/shadow"}`))
	assert.ErrorContains(t, err, "backing file")
	_, err = parseImageInfo([]byte(`{"format": "qcow2", "format-specific": {"data": {"data-file": "/dev/sda"}}}`))
	assert.ErrorContains(t, err, "data file")
	_, err = parseImageInfo([]byte(`{"format": "vmdk"}`))
	assert.ErrorContains(t, err, "qcow2 or raw")
}

// A follower runs no command of the keywords: it would write the image over a
// disk it may share with the leader, and start a second guest on it.
func TestProvisionAsFollowerLeavesAGuestToTheLeader(t *testing.T) {
	r := &T{Image: "/does/not/exist", VCPUs: 1}
	r.SetLoggerForTest(plog.NewDefaultLogger())
	assert.NoError(t, r.ProvisionAsFollower(context.Background()))
}

func TestWithHypervisorKey(t *testing.T) {
	key := "opensvc-hypervisor-key=AB=="
	args := []string{"virt-install", "--vsock", "cid.auto=yes"}
	assert.Equal(t, []string{"virt-install", "--vsock", "cid.auto=yes", "--sysinfo", "oemStrings.entry0=" + key}, withHypervisorKey(args, key))
	assert.Len(t, args, 3, "the keyword value is left as it is")

	theirs := []string{"virt-install", "--sysinfo", "host"}
	assert.Equal(t, theirs, withHypervisorKey(theirs, key), "the start adds it to what the command says")
	assert.Equal(t, args, withHypervisorKey(args, ""), "no key to say")
}

func TestDefinitionWithHypervisorKey(t *testing.T) {
	const definition = `<domain type='kvm'>
  <uuid>de857c8e-280f-49ba-a9e5-b488688f2c8e</uuid>
  <os firmware='efi'>
    <type arch='x86_64' machine='pc-q35-8.2'>hvm</type>
    <boot dev='hd'/>
  </os>
</domain>
`
	assert.Equal(t, "de857c8e-280f-49ba-a9e5-b488688f2c8e", domainUUID([]byte(definition)))
	assert.Empty(t, domainUUID([]byte("<domain/>")))

	first := "opensvc-hypervisor-key=AB+/cd=="
	after, err := definitionWithHypervisorKey([]byte(definition), first)
	require.NoError(t, err)
	s := string(after)
	assert.Contains(t, s, "<sysinfo type='smbios'>\n    <oemStrings>\n      <entry>"+first+"</entry>\n    </oemStrings>\n  </sysinfo>\n  <os firmware='efi'>")
	assert.Contains(t, s, "<boot dev='hd'/>\n    <smbios mode='sysinfo'/>\n  </os>")

	again, err := definitionWithHypervisorKey(after, first)
	require.NoError(t, err)
	assert.Equal(t, s, string(again), "it is written once")

	rotated, err := definitionWithHypervisorKey(after, "opensvc-hypervisor-key=EF==")
	require.NoError(t, err)
	assert.Contains(t, string(rotated), "<entry>opensvc-hypervisor-key=EF==</entry>")
	assert.NotContains(t, string(rotated), first)

	theirs := `<domain>
  <sysinfo type='smbios'>
    <oemStrings>
      <entry>theirs</entry>
    </oemStrings>
  </sysinfo>
  <os>
    <smbios mode='host'/>
  </os>
</domain>
`
	after, err = definitionWithHypervisorKey([]byte(theirs), first)
	require.NoError(t, err)
	assert.Contains(t, string(after), "<oemStrings>\n      <entry>"+first+"</entry>\n      <entry>theirs</entry>")
	assert.Contains(t, string(after), "<smbios mode='sysinfo'/>")
	assert.NotContains(t, string(after), "mode='host'")

	sysinfo := "<domain>\n  <sysinfo type='smbios'>\n    <bios/>\n  </sysinfo>\n  <os>\n  </os>\n</domain>\n"
	after, err = definitionWithHypervisorKey([]byte(sysinfo), first)
	require.NoError(t, err)
	assert.Contains(t, string(after), "<sysinfo type='smbios'>\n    <oemStrings>\n      <entry>"+first+"</entry>\n    </oemStrings>\n    <bios/>")

	_, err = definitionWithHypervisorKey([]byte("<domain/>"), first)
	assert.Error(t, err)
}

// The port has a default: a guest with no vsock device is not touched, a qemu
// too old for vsock also refusing the OEM strings at the start.
func TestDoHypervisorKeyLeavesAGuestWithNoDevice(t *testing.T) {
	saved := hostVsockDevice
	hostVsockDevice = filepath.Join(t.TempDir(), "vhost-vsock")
	defer func() { hostVsockDevice = saved }()
	require.NoError(t, os.WriteFile(hostVsockDevice, nil, 0o600))

	dir := t.TempDir()
	r := &T{VsockPort: 1215}
	r.Name = "guest"
	savedDir := configDir
	configDir = dir
	defer func() { configDir = savedDir }()
	definition := "<domain>\n  <os>\n  </os>\n</domain>\n"
	require.NoError(t, os.WriteFile(r.configFile(), []byte(definition), 0o600))

	r.doHypervisorKey()

	b, err := os.ReadFile(r.configFile())
	require.NoError(t, err)
	assert.Equal(t, definition, string(b))
}
