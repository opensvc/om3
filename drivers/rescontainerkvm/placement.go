package rescontainerkvm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/util/systemd"
)

// systemdMisplacingRelease is the systemd release that places a machine out
// of the slice asked for it, on and off.
//
// libvirt asks systemd-machined for the machine of a guest in the slice of
// its partition, and machined adds the property to its own Slice=machine.slice
// when it starts the scope. systemd 249 keeps both slices as dependencies of
// the scope, and reads its slice as whichever comes first in a hashmap
// ordered by the addresses of the units: a guest lands in the slice of its
// resource or in machine.slice, out of every cap, as the units loaded since
// the last daemon-reload happen to be laid out. The releases before it keep
// one slice, and the ones after remove the former slice when one is set.
const systemdMisplacingRelease = 249

var (
	// procDir is where the kernel says the group of a process, and
	// guestPIDDirs where libvirt keeps the pid file of a running guest.
	// Variables for the tests.
	procDir      = "/proc"
	guestPIDDirs = []string{"/run/libvirt/qemu", "/var/run/libvirt/qemu"}

	// systemdVersion is the release of the systemd of the node, 0 when it
	// can not be read, asked once: it does not change under a process.
	systemdVersion = sync.OnceValue(func() int {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		n, _ := systemd.Version(ctx)
		return n
	})
)

// partitionsRefusal says why a node whose libvirt takes partitions is not
// granted the partitions capability all the same, and nothing when it is:
// what its systemd would do of a guest placed in one. A systemd release that
// can not be read, 0, refuses nothing, the placement being checked after
// each start.
//
// The cgroup v1 hierarchy refuses nothing. The machine of a guest activates
// the slice units of its resource, of its object and of its namespace, which
// systemd then realizes from their properties, at the start of the guest and
// at every daemon-reload: the caps hold since the pg package gives them to
// systemd as these properties, where the ones written in the files alone were
// lifted, as seen with a guest on rhel8 and its systemd 239. The pg_vmem_limit,
// pg_cpus and pg_mems of a group have no property on that hierarchy and stay
// in its files, which systemd writes none of there, so its realizing the
// slice leaves them as om wrote them.
func partitionsRefusal(systemdRelease int) string {
	if systemdRelease == systemdMisplacingRelease {
		return fmt.Sprintf("systemd %d places a guest out of the slice asked for it, on and off", systemdRelease)
	}
	return ""
}

// uncappableReason says why this node can not cap a guest, for the error
// refusing one counted on to be capped.
func uncappableReason() string {
	if !systemd.HasSystemd() {
		return "it has no systemd"
	}
	if s := partitionsRefusal(systemdVersion()); s != "" {
		return s
	}
	return "its libvirt has no partitions"
}

// systemdHierarchy names, in /proc/<pid>/cgroup, the v1 hierarchy systemd
// keeps its units in, with no controller.
const systemdHierarchy = "name=systemd"

// groupOf returns the group systemd placed a process in, read from its
// /proc/<pid>/cgroup, and empty when the content says none.
//
// On a node of the unified hierarchy, and of the hybrid one, it is the path
// of the "0::<path>" line. A node of the v1 hierarchy has no such line, and
// the group is the path of the "<n>:name=systemd:<path>" one, the hierarchy
// systemd keeps the slices and the scope of a machine in: the ones of the
// controllers follow it for the controllers systemd enabled on the units
// only, and say less. The kernel shows a "0::/" line too on a v1 node where
// something mounted a cgroup2 filesystem, with every process at its root: the
// root is not read as the group of the process when systemd says another.
func groupOf(b []byte) string {
	var unified, v1 string
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.SplitN(line, ":", 3)
		if len(l) != 3 {
			continue
		}
		switch {
		case l[0] == "0" && l[1] == "":
			unified = strings.TrimSpace(l[2])
		case l[1] == systemdHierarchy:
			v1 = strings.TrimSpace(l[2])
		}
	}
	if unified == "" || (unified == "/" && v1 != "") {
		return v1
	}
	return unified
}

// unescapeGroup reads the \xNN escapes systemd names its units with, so two
// writings of a group compare as the group they name.
func unescapeGroup(s string) string {
	return systemdHexRe.ReplaceAllStringFunc(s, func(s string) string {
		b, err := strconv.ParseUint(s[2:], 16, 8)
		if err != nil {
			return s
		}
		return string(rune(b))
	})
}

// inGroup says the group a process is in is the group id, or one under it,
// as the scope of the machine of a guest is under the slice of its resource.
func inGroup(group, id string) bool {
	group = filepath.Clean("/" + unescapeGroup(group))
	id = filepath.Clean("/" + unescapeGroup(id))
	return group == id || strings.HasPrefix(group, id+"/")
}

// guestPID returns the pid of the qemu of the guest, from the pid file
// libvirt keeps while it runs, and 0 when there is none to read.
func (t *T) guestPID() int {
	for _, dir := range guestPIDDirs {
		b, err := os.ReadFile(filepath.Join(dir, t.Name+".pid"))
		if err != nil {
			continue
		}
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

// placementAsked says the definition of the guest holds the partition om
// writes in it, one under /opensvc: libvirt was asked for the guest in the
// process group of its resource.
//
// The start of om writes it on a node that places guests, and removes it on
// one that does not, see doPartitions. A definition with none is the one of
// a guest the sysadmin defined and started, as with virt-install, or of a
// definition that can not be read: its guest runs where libvirt places the
// machines, as it did before om read the placement, and nothing was asked
// that its group could be told out of.
func (t *T) placementAsked() bool {
	b, err := os.ReadFile(t.configFile())
	if err != nil {
		return false
	}
	return omPartitionRe.Match(b)
}

// misplacedIn returns the group the guest runs in when it is not the process
// group of the resource nor one under it, and empty when it is, when the
// node does not place guests, when the definition of the guest asks for no
// placement, see placementAsked, or when it can not be told: no guest
// running, no group read in what the kernel says of qemu.
//
// om asks libvirt for the placement and libvirt asks systemd, so a guest out
// of its group is one neither reports: the caps of its resource, of its
// object and of its namespace are applied, and hold nothing. It is what
// systemd 249 does of a placement asked, where a guest whose definition asks
// for none is the choice of its sysadmin, and no misplacement.
func (t *T) misplacedIn() string {
	if t.partition() == "" || !t.placementAsked() {
		return ""
	}
	pid := t.guestPID()
	if pid == 0 {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(procDir, strconv.Itoa(pid), "cgroup"))
	if err != nil {
		return ""
	}
	group := groupOf(b)
	if group == "" || inGroup(group, t.GetPG().ID) {
		return ""
	}
	return group
}

// placementError says the guest runs out of its process group while a cap
// or a claim counts on it being there: the caps are reported applied, and
// the claims count the guest as capped, while nothing holds it.
//
// Out of its group with nothing counting on it, the guest is left running,
// and its status warns. A guest whose definition asks for no placement is
// neither failed nor warned of, see misplacedIn.
func (t *T) placementError() error {
	group := t.misplacedIn()
	if group == "" {
		return nil
	}
	what, err := t.cappedBy()
	if err != nil {
		return err
	}
	if what == "" {
		t.Log().Warnf("the guest runs in %s, out of its process group %s", group, t.GetPG().ID)
		return nil
	}
	return fmt.Errorf("%s, and the guest runs in %s, out of its process group %s, where nothing caps it", what, group, t.GetPG().ID)
}
