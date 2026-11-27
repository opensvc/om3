package rescontainerkvm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/testhelper"
)

const (
	testGuestGroup = "/opensvc.slice/opensvc-ns.ns1.slice/opensvc-ns.ns1-svc.vm.slice/opensvc-ns.ns1-svc.vm-container.1.slice"
	testGuestScope = `machine-qemu\x2d1\x2dvm1.scope`
)

// systemd 249 places a machine in the slice asked or in machine.slice, on and
// off, so a node running it is not granted the partitions capability: a
// guest counted on to be capped is refused there, where it ran uncapped one
// start out of two. The releases around it place the machine where asked.
func TestPartitionsAreNotGrantedOnSystemd249(t *testing.T) {
	assert.Contains(t, partitionsRefusal(249), "systemd 249 places a guest out of the slice asked")
	for _, release := range []int{219, 239, 248, 250, 252, 255, 257} {
		assert.Empty(t, partitionsRefusal(release), "systemd %d", release)
	}
	assert.Empty(t, partitionsRefusal(0), "a release that can not be read refuses nothing")
}

// The group of a process is read where systemd placed it: the unified
// hierarchy, on a unified and on a hybrid node, and the name=systemd one on a
// node of the v1 hierarchy, which has no other saying the scope.
func TestGroupOf(t *testing.T) {
	scope := testGuestGroup + "/" + testGuestScope
	cases := map[string]struct {
		content string
		want    string
	}{
		"the unified hierarchy": {
			content: "0::" + scope + "\n",
			want:    scope,
		},
		"the root group": {
			content: "0::/\n",
			want:    "/",
		},
		"a hybrid node": {
			content: "12:memory:/machine.slice\n1:name=systemd:/machine.slice/x.scope\n0::/machine.slice/x.scope\n",
			want:    "/machine.slice/x.scope",
		},
		"a hybrid node reads the unified line": {
			content: "1:name=systemd:/machine.slice/x.scope\n0::" + scope + "\n",
			want:    scope,
		},
		"the v1 hierarchy": {
			content: "12:memory:/machine.slice\n3:cpu,cpuacct:/machine.slice\n1:name=systemd:/machine.slice/x.scope\n",
			want:    "/machine.slice/x.scope",
		},
		"the v1 hierarchy of rhel7": {
			content: "11:pids:/\n10:memory:" + testGuestGroup + "\n4:cpuacct,cpu:" + testGuestGroup + "\n1:name=systemd:" + scope + "\n",
			want:    scope,
		},
		"the v1 hierarchy with a cgroup2 filesystem mounted": {
			content: "10:memory:" + testGuestGroup + "\n1:name=systemd:" + scope + "\n0::/\n",
			want:    scope,
		},
		"the v1 hierarchy with no systemd line": {
			content: "12:memory:/machine.slice\n",
			want:    "",
		},
		"a named hierarchy that is not the one of systemd": {
			content: "2:name=other:/x\n",
			want:    "",
		},
		"no content": {
			content: "",
			want:    "",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, groupOf([]byte(tc.content)))
		})
	}
}

func TestInGroup(t *testing.T) {
	assert.True(t, inGroup(testGuestGroup+"/"+testGuestScope, testGuestGroup), "the scope of the machine, in the slice of the resource")
	assert.True(t, inGroup(testGuestGroup, testGuestGroup))
	assert.True(t, inGroup(testGuestGroup+"/", testGuestGroup))
	assert.False(t, inGroup("/machine.slice/"+testGuestScope, testGuestGroup), "machine.slice")
	assert.False(t, inGroup("/opensvc.slice/opensvc-ns.ns1.slice/opensvc-ns.ns1-svc.vm.slice", testGuestGroup), "the slice of the object, not of the resource")
	assert.False(t, inGroup(testGuestGroup[:len(testGuestGroup)-len(".slice")]+"0.slice/x.scope", testGuestGroup), "container#10 is not under container#1")
	assert.True(t, inGroup(`/opensvc.slice/opensvc-my\x2dns.slice/x.scope`, "/opensvc.slice/opensvc-my-ns.slice"), "two writings of one group")
}

// The definitions of a guest: the one om wrote its partition in, at a start
// or with virt-install, the one of a guest defined by hand, with none, and
// the one a sysadmin wrote a partition of another tree in.
const (
	testDefinitionPlaced  = "<domain>\n  <name>vm1</name>\n  <resource>\n    <partition>/opensvc/ns.ns1/svc.vm/container.1</partition>\n  </resource>\n</domain>\n"
	testDefinitionByHand  = "<domain>\n  <name>vm1</name>\n</domain>\n"
	testDefinitionOfAdmin = "<domain>\n  <name>vm1</name>\n  <resource>\n    <partition>/machine/prod</partition>\n  </resource>\n</domain>\n"
)

// newPlacedGuest returns a guest running, as libvirt and the kernel say, in
// the group given, its resource capped as told, and its definition holding
// the partition om writes.
func newPlacedGuest(t *testing.T, group string, memCap int64) *T {
	t.Helper()
	testhelper.Setup(t)
	r, _ := newCappedGuest(t, memCap, claim.Unbounded)
	r.Path = naming.Path{Namespace: "ns1", Kind: naming.KindSvc, Name: "vm"}

	dir := t.TempDir()
	previousProcDir, previousPIDDirs, previousConfigDir := procDir, guestPIDDirs, configDir
	procDir, guestPIDDirs, configDir = filepath.Join(dir, "proc"), []string{filepath.Join(dir, "run")}, filepath.Join(dir, "etc")
	t.Cleanup(func() { procDir, guestPIDDirs, configDir = previousProcDir, previousPIDDirs, previousConfigDir })
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(r.configFile(), []byte(testDefinitionPlaced), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(procDir, "4242"), 0755))
	require.NoError(t, os.MkdirAll(guestPIDDirs[0], 0755))
	require.NoError(t, os.WriteFile(filepath.Join(guestPIDDirs[0], r.Name+".pid"), []byte("4242"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), []byte("0::"+group+"\n"), 0644))
	return r
}

// The qemu of a guest is in the scope of its machine, which libvirt asks
// systemd for in the slice of the resource. Out of it, the caps reported
// applied hold nothing: the start fails when a cap counts on the placement,
// and the status warns when none does.
func TestAGuestOutOfItsProcessGroup(t *testing.T) {
	placed := testGuestGroup + "/" + testGuestScope
	misplaced := "/machine.slice/" + testGuestScope

	t.Run("placed and capped", func(t *testing.T) {
		r := newPlacedGuest(t, placed, 1<<30)
		assert.Empty(t, r.misplacedIn())
		assert.NoError(t, r.placementError())
	})

	t.Run("misplaced and capped", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		assert.Equal(t, misplaced, r.misplacedIn())
		err := r.placementError()
		assert.ErrorContains(t, err, "container#1 is capped on the")
		assert.ErrorContains(t, err, "the guest runs in "+misplaced+", out of its process group "+testGuestGroup)
	})

	t.Run("misplaced with no cap nor claim", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, claim.Unbounded)
		assert.Equal(t, misplaced, r.misplacedIn(), "what the status warns of")
		assert.NoError(t, r.placementError(), "nothing counts on the placement")
	})

	t.Run("misplaced in a namespace claiming the memory", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, claim.Unbounded)
		p := naming.Path{Namespace: "ns1", Kind: naming.KindNscfg, Name: "namespace"}
		require.NoError(t, os.MkdirAll(filepath.Dir(p.ConfigFile()), 0755))
		require.NoError(t, os.WriteFile(p.ConfigFile(), []byte("[claim#1]\ntype = memory\nlimit = 4g\n"), 0644))
		assert.ErrorContains(t, r.placementError(), "the ns1 namespace claims the memory, and the guest runs in "+misplaced)
	})

	// A guest defined by hand, as with virt-install, has no partition of
	// om in its definition: libvirt runs it in machine.slice, which is the
	// choice of its sysadmin and no misplacement. Its status does not warn,
	// which had the object warn instead of up, and its start does not fail,
	// capped or not.
	for name, definition := range map[string]string{
		"defined by hand":                          testDefinitionByHand,
		"defined with the partition of a sysadmin": testDefinitionOfAdmin,
	} {
		t.Run("misplaced and capped, "+name, func(t *testing.T) {
			r := newPlacedGuest(t, misplaced, 1<<30)
			require.NoError(t, os.WriteFile(r.configFile(), []byte(definition), 0644))
			assert.Empty(t, r.misplacedIn(), "no placement was asked: the status does not warn")
			assert.NoError(t, r.placementError(), "and the start does not fail")
		})

		t.Run("misplaced with no cap nor claim, "+name, func(t *testing.T) {
			r := newPlacedGuest(t, misplaced, claim.Unbounded)
			require.NoError(t, os.WriteFile(r.configFile(), []byte(definition), 0644))
			assert.Empty(t, r.misplacedIn(), "no placement was asked: the status does not warn")
			assert.NoError(t, r.placementError())
		})
	}

	t.Run("misplaced and capped, with no definition to read", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		require.NoError(t, os.Remove(r.configFile()))
		assert.Empty(t, r.misplacedIn())
		assert.NoError(t, r.placementError())
	})

	// The same guest is still refused where the node can not cap it: the
	// refusal of a capped guest there reads no placement.
	t.Run("capped and defined by hand, on a node that can not cap it", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		require.NoError(t, os.WriteFile(r.configFile(), []byte(testDefinitionByHand), 0644))
		partitionCapable = func() bool { return false }
		assert.ErrorContains(t, r.memCapError(), "this node can not cap a guest")
	})

	t.Run("a node that places no guest", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		partitionCapable = func() bool { return false }
		assert.Empty(t, r.misplacedIn(), "nothing was asked of systemd")
		assert.NoError(t, r.placementError())
	})

	t.Run("no guest running", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		require.NoError(t, os.Remove(filepath.Join(guestPIDDirs[0], r.Name+".pid")))
		assert.Empty(t, r.misplacedIn())
	})

	// A node of the v1 hierarchy says the scope of the machine in the
	// hierarchy of systemd alone: the placement is read there.
	v1 := func(group string) []byte {
		return []byte("10:memory:" + filepath.Dir(group) + "\n4:cpu,cpuacct:" + filepath.Dir(group) + "\n1:name=systemd:" + group + "\n")
	}

	t.Run("placed and capped on the v1 hierarchy", func(t *testing.T) {
		r := newPlacedGuest(t, placed, 1<<30)
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), v1(placed), 0644))
		assert.Empty(t, r.misplacedIn())
		assert.NoError(t, r.placementError())
	})

	t.Run("misplaced and capped on the v1 hierarchy", func(t *testing.T) {
		r := newPlacedGuest(t, placed, 1<<30)
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), v1(misplaced), 0644))
		assert.Equal(t, misplaced, r.misplacedIn())
		assert.ErrorContains(t, r.placementError(), "the guest runs in "+misplaced+", out of its process group "+testGuestGroup)
	})

	t.Run("misplaced with no cap nor claim on the v1 hierarchy", func(t *testing.T) {
		r := newPlacedGuest(t, placed, claim.Unbounded)
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), v1(misplaced), 0644))
		assert.Equal(t, misplaced, r.misplacedIn(), "what the status warns of")
		assert.NoError(t, r.placementError(), "nothing counts on the placement")
	})

	t.Run("placed on a hybrid node", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), append(v1(placed), []byte("0::"+placed+"\n")...), 0644))
		assert.Empty(t, r.misplacedIn())
	})

	t.Run("misplaced on a hybrid node", func(t *testing.T) {
		r := newPlacedGuest(t, placed, 1<<30)
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), append(v1(misplaced), []byte("0::"+misplaced+"\n")...), 0644))
		assert.Equal(t, misplaced, r.misplacedIn())
	})

	t.Run("no group of systemd to read", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, 1<<30)
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "4242", "cgroup"), []byte("12:memory:/machine.slice\n"), 0644))
		assert.Empty(t, r.misplacedIn(), "neither the unified hierarchy nor the one of systemd")
	})
}

// A pg_* keyword of the configuration of the namespace of the guest is a cap
// counting on the guest being under the slice of the namespace, as
// one of its resource or a claim is: the guest is refused on a node that can
// not place it, and its start fails when systemd placed it elsewhere.
func TestACapOfTheNamespaceConfigurationCountsOnThePlacement(t *testing.T) {
	misplaced := "/machine.slice/" + testGuestScope
	withNamespaceCap := func(r *T, keyword string) {
		r.SetObject(fakeGuestObject{memCap: claim.Unbounded, namespaceCap: claim.Unbounded, pgCapKeyword: keyword})
	}

	t.Run("refused on a node that places no guest", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, claim.Unbounded)
		partitionCapable = func() bool { return false }
		assert.NoError(t, r.memCapError(), "nothing caps the guest")
		withNamespaceCap(r, "pg_cpu_quota")
		assert.ErrorContains(t, r.memCapError(), "the ns1 namespace sets pg_cpu_quota, and this node can not cap a guest: ")
		assert.ErrorContains(t, r.Start(t.Context()), "the ns1 namespace sets pg_cpu_quota, and this node can not cap a guest: ")
	})

	t.Run("the start fails on a guest placed elsewhere", func(t *testing.T) {
		r := newPlacedGuest(t, misplaced, claim.Unbounded)
		assert.NoError(t, r.placementError(), "nothing caps the guest")
		withNamespaceCap(r, "pg_pids_max")
		assert.ErrorContains(t, r.placementError(), "the ns1 namespace sets pg_pids_max, and the guest runs in "+misplaced)
	})

	t.Run("placed under the slice of the namespace", func(t *testing.T) {
		r := newPlacedGuest(t, testGuestGroup+"/"+testGuestScope, claim.Unbounded)
		withNamespaceCap(r, "pg_mem_limit")
		assert.NoError(t, r.placementError())
		assert.NoError(t, r.memCapError())
	})
}

func TestDefinitionWithoutPartition(t *testing.T) {
	const own = "<domain>\n  <name>vm1</name>\n  <resource>\n    <partition>/opensvc/ns.a/svc.b/container.1</partition>\n  </resource>\n  <os/>\n</domain>\n"
	assert.Equal(t, "<domain>\n  <name>vm1</name>\n  <os/>\n</domain>\n", string(definitionWithoutPartition([]byte(own))), "the partition of a process group, and the resource element it leaves empty")

	const shared = "<domain>\n  <resource>\n    <partition>/opensvc/svc.b</partition>\n    <fibrechannel appid='x'/>\n  </resource>\n</domain>\n"
	assert.Equal(t, "<domain>\n  <resource>\n    <fibrechannel appid='x'/>\n  </resource>\n</domain>\n", string(definitionWithoutPartition([]byte(shared))), "the rest of the resource element is kept")

	for name, definition := range map[string]string{
		"the partition of libvirt":    "<domain>\n  <resource>\n    <partition>/machine</partition>\n  </resource>\n</domain>\n",
		"a partition of the sysadmin": "<domain>\n  <resource>\n    <partition>/opensvcx/prod</partition>\n  </resource>\n</domain>\n",
		"no partition":                "<domain>\n  <name>vm1</name>\n</domain>\n",
	} {
		assert.Equal(t, definition, string(definitionWithoutPartition([]byte(definition))), name)
	}
}

// A node that can not place a guest removes, at the start, the partition a
// node that can wrote in the definition, or this one before it lost the
// capability: libvirt went on asking systemd for a placement nothing checked.
// A node that can writes it back.
func TestTheStartRemovesAPartitionTheNodeCanNotHonour(t *testing.T) {
	r, _ := newCappedGuest(t, claim.Unbounded, claim.Unbounded)
	previous := configDir
	configDir = t.TempDir()
	t.Cleanup(func() { configDir = previous })
	const without = "<domain>\n  <name>vm1</name>\n</domain>\n"
	const with = "<domain>\n  <name>vm1</name>\n  <resource>\n    <partition>/opensvc/ns.ns1/svc.vm/container.1</partition>\n  </resource>\n</domain>\n"
	read := func() string {
		b, err := os.ReadFile(r.configFile())
		require.NoError(t, err)
		return string(b)
	}
	require.NoError(t, os.WriteFile(r.configFile(), []byte(with), 0644))

	partitionCapable = func() bool { return false }
	require.NoError(t, r.doPartitions())
	assert.Equal(t, without, read())
	require.NoError(t, r.doPartitions())
	assert.Equal(t, without, read(), "nothing left to remove")

	partitionCapable = func() bool { return true }
	require.NoError(t, r.doPartitions())
	assert.Equal(t, with, read())
}
