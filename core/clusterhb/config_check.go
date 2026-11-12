package clusterhb

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/hbcfg"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

func init() {
	object.RegisterClusterConfigCheck(checkClusterConfig)
}

// checkClusterConfig says the heartbeats of a cluster configuration whose
// address their driver would refuse, on any of their nodes: the keywords are
// scopable, so a heartbeat is checked as each of its nodes reads it.
//
//   - a multicast group naming a zone, or not a multicast one
//   - an IPv6 multicast group with no intf to join it on
//   - an IPv6 link-local unicast address
//   - a unicast heartbeat listening on the port and an address another
//     unicast heartbeat listens on, on a node
//
// The write making one is refused, rather than the heartbeat listed failed
// on the nodes, after the fact.
func checkClusterConfig(cfg *xconfig.T) xconfig.Alerts {
	alerts := checkUnicastListens(cfg)
	for _, section := range cfg.SectionStrings() {
		if !strings.HasPrefix(section, "hb#") {
			continue
		}
		hbType := cfg.GetString(key.New(section, "type"))
		did := driver.NewID(driver.GroupHeartbeat, hbType)
		var check func(node string) (string, error)
		switch hbType {
		case "multicast":
			check = func(node string) (string, error) {
				addr := cfg.GetStringAs(key.New(section, "addr"), node)
				intf := cfg.GetStringAs(key.New(section, "intf"), node)
				err := hbcfg.CheckMulticastGroup(addr, intf)
				if errors.Is(err, hbcfg.ErrIntfRequired) {
					return "intf", err
				}
				return "addr", err
			}
		case "unicast":
			check = func(node string) (string, error) {
				addr := cfg.GetStringAs(key.New(section, "addr"), node)
				return "addr", hbcfg.CheckUnicastAddr(addr)
			}
		default:
			continue
		}
		type problem struct {
			option  string
			comment string
		}
		problems := make([]problem, 0)
		problemNodes := make(map[problem][]string)
		for _, node := range heartbeatNodes(cfg, section) {
			option, err := check(node)
			if err == nil {
				continue
			}
			p := problem{option: option, comment: err.Error()}
			if _, ok := problemNodes[p]; !ok {
				problems = append(problems, p)
			}
			problemNodes[p] = append(problemNodes[p], node)
		}
		for _, p := range problems {
			k := key.New(section, p.option)
			comment := fmt.Sprintf("%s: %s, %s", section, p.comment, onNodes(problemNodes[p]))
			if p.option == "intf" {
				alerts = append(alerts, cfg.NewAlertMissingKeyword(k, did, comment))
			} else {
				alerts = append(alerts, cfg.NewAlertCandidates(k, did, comment))
			}
		}
	}
	return alerts
}

// heartbeatNodes returns the nodes a heartbeat runs on, which are the
// cluster nodes when it does not say.
func heartbeatNodes(cfg *xconfig.T, section string) []string {
	if nodes := cfg.GetStrings(key.New(section, "nodes")); len(nodes) > 0 {
		return nodes
	}
	return cfg.GetStrings(key.New("cluster", "nodes"))
}

func onNodes(nodes []string) string {
	sort.Strings(nodes)
	if len(nodes) == 1 {
		return "on node " + nodes[0]
	}
	return "on nodes " + strings.Join(nodes, ", ")
}

// unicastListen is where the rx of a unicast heartbeat listens on a node.
type unicastListen struct {
	addr string
	port int
}

// checkUnicastListens says the unicast heartbeats listening, on a node of
// both, on the same port of an address both listen on: the rx of the second
// can not bind, and its tx is seen beating, as the rx of the first receives
// its frames, while the cluster has one network path less than it shows.
func checkUnicastListens(cfg *xconfig.T) xconfig.Alerts {
	var alerts xconfig.Alerts
	sections := make([]string, 0)
	for _, section := range cfg.SectionStrings() {
		if strings.HasPrefix(section, "hb#") && cfg.GetString(key.New(section, "type")) == "unicast" {
			sections = append(sections, section)
		}
	}
	sort.Strings(sections)
	did := driver.NewID(driver.GroupHeartbeat, "unicast")
	for i, section := range sections {
		nodes := heartbeatNodes(cfg, section)
		for _, other := range sections[:i] {
			otherNodes := heartbeatNodes(cfg, other)
			conflictNodes := make([]string, 0)
			port := 0
			for _, node := range nodes {
				if !slices.Contains(otherNodes, node) {
					continue
				}
				a, ok := unicastListenOf(cfg, section, node)
				if !ok {
					continue
				}
				b, ok := unicastListenOf(cfg, other, node)
				if !ok || a.port != b.port || !listenAddrsOverlap(a.addr, b.addr) {
					continue
				}
				conflictNodes = append(conflictNodes, node)
				port = a.port
			}
			if len(conflictNodes) == 0 {
				continue
			}
			comment := fmt.Sprintf("%s: the port %d is the one %s listens on, %s: use another port, or another addr on each node", section, port, other, onNodes(conflictNodes))
			alerts = append(alerts, cfg.NewAlertCandidates(key.New(section, "port"), did, comment))
		}
	}
	return alerts
}

// unicastListenOf returns where the rx of the unicast heartbeat section
// listens on node, as node reads its keywords.
func unicastListenOf(cfg *xconfig.T, section, node string) (unicastListen, bool) {
	v, err := cfg.EvalAs(key.New(section, "port"), node)
	if err != nil {
		return unicastListen{}, false
	}
	port, ok := v.(int)
	if !ok {
		return unicastListen{}, false
	}
	return unicastListen{addr: cfg.GetStringAs(key.New(section, "addr"), node), port: port}, true
}

// listenAddrsOverlap says two listen addresses share an address: one of them
// is empty or unspecified, listening on every address of the node, or they
// are the same address, written in any of its forms.
func listenAddrsOverlap(a, b string) bool {
	if a == "" || b == "" {
		return true
	}
	ipA, errA := netip.ParseAddr(a)
	ipB, errB := netip.ParseAddr(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(a, b)
	}
	if ipA.IsUnspecified() || ipB.IsUnspecified() {
		return true
	}
	return ipA.Unmap() == ipB.Unmap()
}
