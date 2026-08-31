package dns

import (
	"fmt"
	"net"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
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

// recordKey uniquely identifies a DNS record (excluding TTL and DomainID which are metadata)
type recordKey struct {
	Name    string
	Type    string
	Content string
}

// Key returns the identity of the record, the part of it that TTL and
// DomainID changes don't affect.
func (t Record) Key() recordKey {
	return recordKey{t.Name, t.Type, t.Content}
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
	// Refresh the indexed SOA/NS records, they depend on clusterConfig.DNS
	t.setClusterRecords()
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
	if recordMap, ok := t.state[key]; ok {
		for _, record := range recordMap {
			t.pubDeleted(record, c.Path, c.Node)
		}
		t.setStateRecords(key, nil)
	}
}

func (t *Manager) onInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	key := t.stateKey(c.Path, c.Node)
	name := naming.NewFQDN(c.Path, t.clusterConfig.Name).String() + "."
	nameOnNode := fmt.Sprintf("%s.%s.%s.%s.node.%s.", c.Path.Name, c.Path.Namespace, c.Path.Kind, c.Node, t.clusterConfig.Name)
	newRecordsMap := make(map[recordKey]Record)
	existingRecordsMap := t.state[key]

	stage := func(record Record) {
		recKey := record.Key()

		// Check if this record already exists (by identity, not by TTL/DomainID)
		if existingRecord, ok := existingRecordsMap[recKey]; !ok {
			// New record, publish update
			t.pubUpdated(record, c.Path, c.Node)
		} else if existingRecord != record {
			// Record exists but has changed (TTL or DomainID difference)
			t.pubUpdated(record, c.Path, c.Node)
		}
		// Store in new records map (preserves the full Record with current TTL/DomainID)
		newRecordsMap[recKey] = record
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
		if !rstat.Status.Is(status.Up) {
			continue
		}
		i, ok := rstat.Info[ipAddrInfoKey]
		if !ok {
			continue
		}
		ipAddr, ok := i.(string)
		if !ok || ipAddr == "" {
			continue
		}
		ip := net.ParseIP(ipAddr)
		if ip == nil {
			continue
		}
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

	// Delete records that no longer exist
	for recordKey, existingRecord := range existingRecordsMap {
		if _, ok := newRecordsMap[recordKey]; !ok {
			t.pubDeleted(existingRecord, c.Path, c.Node)
		}
	}

	t.setStateRecords(key, newRecordsMap)
}

func (t *Manager) onCmdGet(c cmdGet) {
	// Use nameIndex for O(1) lookup
	records, ok := t.nameIndex[c.Name]
	if !ok {
		c.errC <- nil
		c.resp <- Zone{}
		return
	}
	// Pre-size slice with estimated capacity (Fix 3)
	zone := make(Zone, 0, len(records))
	seen := make(map[recordKey]bool)
	for _, record := range records {
		if (c.Type != "ANY") && (record.Type != c.Type) {
			continue
		}
		key := record.Key()
		if !seen[key] {
			zone = append(zone, record)
			seen[key] = true
		}
	}
	c.errC <- nil
	c.resp <- zone
}

func (t *Manager) onCmdGetZone(c cmdGetZone) {
	c.errC <- nil
	c.resp <- t.zone()
}

func (t *Manager) zone() Zone {
	zone := t.clusterRecordZone()
	seen := make(map[recordKey]bool)
	for _, recordMap := range t.state {
		for _, record := range recordMap {
			key := record.Key()
			if !seen[key] {
				zone = append(zone, record)
				seen[key] = true
			}
		}
	}
	return zone
}

// clusterRecordZone returns the cluster level records: the zone SOA, and
// the NS and A records of each configured nameserver.
func (t *Manager) clusterRecordZone() Zone {
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
	return zone
}

// setClusterRecords replaces the cluster level records in the name index
// with the ones the current cluster config yields.
func (t *Manager) setClusterRecords() {
	for _, record := range t.clusterRecords {
		t.delIndexRecord(record)
	}
	t.clusterRecords = t.clusterRecordZone()
	for _, record := range t.clusterRecords {
		t.addIndexRecord(record)
	}
}

// setStateRecords replaces the records of a state key, and keeps the name
// index in sync: only the records of this key are reindexed, so the cost
// doesn't grow with the number of objects in the cluster.
//
// A nil recordMap drops the state key.
func (t *Manager) setStateRecords(key stateKey, recordMap map[recordKey]Record) {
	for _, record := range t.state[key] {
		t.delIndexRecord(record)
	}
	for _, record := range recordMap {
		t.addIndexRecord(record)
	}
	if len(recordMap) > 0 {
		t.state[key] = recordMap
	} else {
		delete(t.state, key)
	}
}

// addIndexRecord adds a record to the name index
func (t *Manager) addIndexRecord(record Record) {
	t.nameIndex[record.Name] = append(t.nameIndex[record.Name], record)
}

// delIndexRecord removes one occurrence of record from the name index.
// The same record can be indexed more than once, when several state keys
// or several nameservers yield it, so the other occurrences are kept.
func (t *Manager) delIndexRecord(record Record) {
	records, ok := t.nameIndex[record.Name]
	if !ok {
		return
	}
	key := record.Key()
	for i, indexed := range records {
		if indexed.Key() != key {
			continue
		}
		records[i] = records[len(records)-1]
		records = records[:len(records)-1]
		if len(records) == 0 {
			delete(t.nameIndex, record.Name)
		} else {
			t.nameIndex[record.Name] = records
		}
		return
	}
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
