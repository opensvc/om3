package dns

import (
	"fmt"
	"net"
	"time"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

const hexDigit = "0123456789abcdef"

var (
	hostnameInfoKey = "hostname"
	exposeInfoKey   = "expose"
	ipAddrInfoKey   = "ipaddr"

	// SOA records properties
	contact = "contact@opensvc.com"
	serial  = 1
	refresh = 7200
	retry   = 3600
	expire  = 432000
	minimum = 86400

	defaultPrio   = 0
	defaultWeight = 10
)

func (t *Manager) stateKey(p naming.Path, node string) stateKey {
	return stateKey{
		path: p.String(),
		node: node,
	}
}

func (t *Manager) onNodeStatsUpdated(c *msgbus.NodeStatsUpdated) {
	t.score[c.Node] = c.Value.Score
}

func (t *Manager) onClusterConfigUpdated(c *msgbus.ClusterConfigUpdated) {
	t.clusterConfig = c.Value
	change, err := t.sockChown()
	if err != nil {
		// TODO: change status.state to warning ? for om mon -w
		t.log.Errorf("sock chown error: %s", err)
	}
	if change {
		t.status.ConfiguredAt = time.Now()
	}
	if len(t.clusterConfig.DNS) != len(t.status.Nameservers) {
		change = true
	} else {
		for i := 0; i < len(t.status.Nameservers); i++ {
			if t.clusterConfig.DNS[i] != t.status.Nameservers[i] {
				change = true
				break
			}
		}
	}
	if change {
		t.publishSubsystemDnsUpdated()
	}
}

func (t *Manager) pubDeleted(record Record, p naming.Path, node string) {
	t.publisher.Pub(&msgbus.ZoneRecordDeleted{
		Path:    p,
		Node:    node,
		Name:    record.Name,
		Type:    record.Type,
		TTL:     record.TTL,
		Content: record.Content,
	}, pubsub.Label{"node", node}, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
}

func (t *Manager) pubUpdated(record Record, p naming.Path, node string) {
	t.publisher.Pub(&msgbus.ZoneRecordUpdated{
		Path:    p,
		Node:    node,
		Name:    record.Name,
		Type:    record.Type,
		TTL:     record.TTL,
		Content: record.Content,
	}, pubsub.Label{"node", node}, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()})
}

func (t *Manager) onInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	key := t.stateKey(c.Path, c.Node)
	if records, ok := t.state[key]; ok {
		for _, record := range records {
			t.pubDeleted(record, c.Path, c.Node)
		}
		delete(t.state, key)
	}
}

func (t *Manager) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	key := t.stateKey(c.Path, c.Node)
	name := naming.NewFQDN(c.Path, t.clusterConfig.Name).String() + "."
	nameOnNode := fmt.Sprintf("%s.%s.%s.%s.node.%s.", c.Path.Name, c.Path.Namespace, c.Path.Kind, c.Node, t.clusterConfig.Name)
	records := make(Zone, 0)
	updatedRecords := make(map[string]any)
	existingRecords := t.getExistingRecords(key)
	stage := func(record Record) {
		records = append(records, record)
		existingRecord, ok := existingRecords[record.Name]
		var change bool
		switch {
		case !ok:
			change = true
		case existingRecord.Content != record.Content:
			change = true
		case existingRecord.Type != record.Type:
			change = true
		case existingRecord.DomainID != record.DomainID:
			change = true
		case existingRecord.TTL != record.TTL:
			change = true
		}
		if change {
			t.pubUpdated(record, c.Path, c.Node)
			updatedRecords[record.Name] = nil
		}
	}
	stageSRV := func(s string) error {
		expose, err := ParseExpose(s)
		if err != nil {
			return err
		}
		var weight int
		if i, ok := t.score[c.Node]; ok {
			weight = int(i)
		} else {
			weight = defaultWeight
		}
		stage(Record{
			Name:     fmt.Sprintf("_%d._%s.%s", expose.FrontendPort, expose.Network, name),
			DomainID: -1,
			Type:     "SRV",
			TTL:      60,
			Content:  fmt.Sprintf("%d %d %d %s", defaultPrio, weight, expose.BackendPort, nameOnNode),
		})
		return nil
	}
	stageSRVs := func(rid string, r resource.Status) {
		i, ok := r.Info[exposeInfoKey]
		if !ok {
			return
		}
		switch exposes := i.(type) {
		case []any:
			for _, expose := range exposes {
				if s, ok := expose.(string); ok {
					if err := stageSRV(s); err != nil {
						t.log.Warnf("%s: %s: parse %s=%s: %s", c.Path, rid, exposeInfoKey, s, err)
					}
				}
			}
		}
	}
	for rid, rstat := range c.Value.Resources {
		i, ok := rstat.Info[ipAddrInfoKey]
		if !ok {
			continue
		}
		ipAddr, ok := i.(string)
		if !ok {
			continue
		}
		ip := net.ParseIP(ipAddr)
		isIPV4 := ip.To4() != nil
		var aType, ptrType string
		if isIPV4 {
			aType = "A"
			ptrType = "PTR"
		} else {
			aType = "AAAA"
			ptrType = "PTR"
		}
		getResNames := func() (string, string) {
			if i, ok := rstat.Info[hostnameInfoKey]; ok {
				hostname, _ := i.(string)
				if hostname != "" {
					resName := hostname + "." + name
					resNameOnNode := hostname + "." + nameOnNode
					return resName, resNameOnNode
				}
			}
			if id, err := resourceid.Parse(rid); err == nil {
				resName := id.Index() + "." + name
				resNameOnNode := id.Index() + "." + nameOnNode
				return resName, resNameOnNode
			}
			return "", ""
		}
		resName, resNameOnNode := getResNames()

		// Add a direct record (node agnostic)
		stage(Record{
			Name:     name,
			DomainID: -1,
			Type:     aType,
			TTL:      60,
			Content:  ipAddr,
		})
		if resName != "" {
			stage(Record{
				Name:     resName,
				DomainID: -1,
				Type:     aType,
				TTL:      60,
				Content:  ipAddr,
			})
			// Add a reverse record (node agnostic)
			stage(Record{
				Name:     reverseAddr(ip),
				DomainID: -1,
				Type:     ptrType,
				TTL:      60,
				Content:  resName,
			})
		} else {
			// Add a reverse record (node agnostic)
			stage(Record{
				Name:     reverseAddr(ip),
				DomainID: -1,
				Type:     ptrType,
				TTL:      60,
				Content:  name,
			})
		}

		// Add a direct record (node affine)
		stage(Record{
			Name:     nameOnNode,
			DomainID: -1,
			Type:     aType,
			TTL:      60,
			Content:  ipAddr,
		})
		if resNameOnNode != "" {
			stage(Record{
				Name:     resNameOnNode,
				DomainID: -1,
				Type:     aType,
				TTL:      60,
				Content:  ipAddr,
			})
			// Add a reverse record (node affine)
			stage(Record{
				Name:     reverseAddr(ip),
				DomainID: -1,
				Type:     ptrType,
				TTL:      60,
				Content:  resNameOnNode,
			})
		} else {
			// Add a reverse record (node affine)
			stage(Record{
				Name:     reverseAddr(ip),
				DomainID: -1,
				Type:     ptrType,
				TTL:      60,
				Content:  nameOnNode,
			})
		}

		stageSRVs(rid, rstat)
	}

	for key, record := range existingRecords {
		if _, ok := updatedRecords[key]; !ok {
			t.pubDeleted(record, c.Path, c.Node)
		}
	}
	if len(records) > 0 {
		t.state[key] = records
	} else {
		delete(t.state, key)
	}
}

func (t *Manager) onCmdGet(c cmdGet) {
	zone := make(Zone, 0)
	for _, record := range t.zone() {
		if record.Name != c.Name {
			continue
		}
		if (c.Type != "ANY") && (record.Type != c.Type) {
			continue
		}
		zone = append(zone, record)
	}
	c.errC <- nil
	c.resp <- zone
}

func (t *Manager) onCmdGetZone(c cmdGetZone) {
	c.errC <- nil
	c.resp <- t.zone()
}

func (t *Manager) zone() Zone {
	zone := make(Zone, 0)
	zoneName := t.clusterConfig.Name + "."
	for i, dns := range t.clusterConfig.DNS {
		nsName := fmt.Sprintf("ns%d.%s", i+1, zoneName)
		soaContent := fmt.Sprintf("dns.%s %s %d %d %d %d %d", zoneName, contact, serial, refresh, retry, expire, minimum)
		zone = append(zone,
			Record{
				Name:     zoneName,
				DomainID: -1,
				Type:     "SOA",
				TTL:      60,
				Content:  soaContent,
			},
			Record{
				Name:     nsName,
				DomainID: -1,
				Type:     "A",
				TTL:      60,
				Content:  dns,
			},
			Record{
				Name:     zoneName,
				DomainID: -1,
				Type:     "NS",
				TTL:      3600,
				Content:  nsName,
			},
		)
	}
	for _, records := range t.state {
		zone = append(zone, records...)
	}
	return zone
}

func (t *Manager) getExistingRecords(key stateKey) map[string]Record {
	m := make(map[string]Record)
	records, ok := t.state[key]
	if !ok {
		return m
	}
	for _, record := range records {
		m[record.Name] = record
	}
	return m
}

func uitoa(val uint) string {
	if val == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf) - 1
	for val >= 10 {
		q := val / 10
		buf[i] = byte('0' + val - q*10)
		i--
		val = q
	}
	buf[i] = byte('0' + val)
	return string(buf[i:])
}

func reverseAddr(ip net.IP) (arpa string) {
	if ip.To4() != nil {
		return uitoa(uint(ip[15])) + "." + uitoa(uint(ip[14])) + "." + uitoa(uint(ip[13])) + "." + uitoa(uint(ip[12])) + ".in-addr.arpa."
	}

	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	for i := len(ip) - 1; i >= 0; i-- {
		v := ip[i]
		buf = append(buf, hexDigit[v&0xF])
		buf = append(buf, '.')
		buf = append(buf, hexDigit[v>>4])
		buf = append(buf, '.')
	}
	buf = append(buf, "ip6.arpa."...)
	return string(buf)
}
