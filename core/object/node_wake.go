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

// Wake sends, from the local node, the magic packets waking the cluster node
// nodename, as configured in its wake#<nodename> section. The macs, when
// set, replace the wake#<nodename>.mac addresses.
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
	if len(macs) == 0 {
		return fmt.Errorf("node %s has no mac address: set %s.mac", nodename, section)
	}
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
