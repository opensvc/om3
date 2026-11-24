package object

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/opensvc/om3/v3/util/getaddr"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/wakeonlan"
)

// wakeLookup and wakeBroadcastFor are replaced in tests.
var (
	wakeLookup       = getaddr.Lookup
	wakeBroadcastFor = wakeonlan.BroadcastFor
)

// Wake wakes the cluster node nodename from the local node.
//
// It asks the collector first, for a virtual machine node: the collector
// queues the start of the container of its hypervisor running it, found by
// the machine uuid the node reported. When the collector can not, as for a
// physical node, it sends the magic packets configured in the
// wake#<nodename> section. The macs, when set, replace the
// wake#<nodename>.mac addresses.
//
// Without wake#<nodename>.broadcast, the packet goes to the broadcast address
// of the local interface on the subnet of the address nodename resolves to,
// the cached one when the dns does not answer.
//
// Waking the local node does nothing: it is awake.
func (t *Node) Wake(nodename string, macs []string) error {
	if nodename == "" {
		return fmt.Errorf("node name is not set")
	}
	if nodename == hostname.Hostname() {
		t.log.Infof("node %s is the local node: already awake", nodename)
		return nil
	}
	nodenames, err := t.Nodes()
	if err != nil {
		return err
	}
	if !slices.Contains(nodenames, nodename) {
		return fmt.Errorf("node %s is not a cluster node", nodename)
	}
	section := "wake#" + nodename
	if len(macs) == 0 {
		macs = t.wakeList(section, "mac")
	}
	ans, collectorErr := wakeByCollector(t, nodename)
	if collectorErr == nil {
		if ans.Queued {
			t.log.Infof("wake node %s: the collector queued the start of %s %s on %s (action %d)", nodename, ans.Path, ans.RID, ans.Hypervisor, ans.ActionID)
		} else {
			t.log.Infof("wake node %s: the start of %s %s on %s is already queued (action %d)", nodename, ans.Path, ans.RID, ans.Hypervisor, ans.ActionID)
		}
		return nil
	}
	if len(macs) == 0 {
		return fmt.Errorf("node %s: the collector can not wake it: %w, and it has no mac address to send a magic packet to: set %s.mac", nodename, collectorErr, section)
	}
	t.log.Infof("wake node %s: the collector can not wake it: %s: send magic packets", nodename, collectorErr)
	ports, err := t.wakePorts(section)
	if err != nil {
		return err
	}
	broadcasts := t.wakeList(section, "broadcast")
	if len(broadcasts) == 0 {
		broadcast, err := t.wakeBroadcast(nodename)
		if err != nil {
			return fmt.Errorf("%w: set %s.broadcast", err, section)
		}
		broadcasts = []string{broadcast}
	}
	t.log.Infof("wake node %s", nodename)
	return t.Wol(macs, broadcasts, ports)
}

func (t *Node) wakePorts(section string) ([]int, error) {
	var ports []int
	for _, s := range t.wakeList(section, "port") {
		port, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("%s.port: invalid port %q", section, s)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func (t *Node) wakeBroadcast(nodename string) (string, error) {
	ip, age, err := wakeLookup(nodename)
	if err != nil {
		return "", fmt.Errorf("resolve node %s address: %w", nodename, err)
	}
	if age > 0 {
		t.log.Infof("node %s address %s from the cache, %s old", nodename, ip, age)
	}
	broadcast, err := wakeBroadcastFor(ip)
	if err != nil {
		return "", fmt.Errorf("node %s address %s: %w", nodename, ip, err)
	}
	return broadcast.String(), nil
}

// wakeList returns the words of a wake#<nodename> list keyword. Commas
// separate them too, as in the om node wol and om node wake flags: a mac
// address, a broadcast address and a port have none.
func (t *Node) wakeList(section, option string) []string {
	return strings.FieldsFunc(t.mergedConfig.GetString(key.New(section, option)), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}
