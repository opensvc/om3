package clusterhb

import (
	"errors"
	"fmt"
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
//
// The write making one is refused, rather than the heartbeat listed failed
// on the nodes, after the fact.
func checkClusterConfig(cfg *xconfig.T) xconfig.Alerts {
	var alerts xconfig.Alerts
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
