package dns

import (
	"fmt"
	"net"

	"opensvc.com/opensvc/core/fqdn"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/daemon/msgbus"
)

const hexDigit = "0123456789abcdef"

var (
	ipAddrInfoKey = "ipaddr"
)

func (t *dns) onClusterConfigUpdated(c msgbus.ClusterConfigUpdated) {
	t.cluster = c.Value
}

func (t *dns) pubDeleted(record Record) {
	t.bus.Pub(msgbus.ZoneRecordDeleted{
		Name:  record.Name,
		Class: record.Class,
		Type:  record.Type,
		TTL:   record.TTL,
		Data:  record.Data,
	})
}

func (t *dns) pubUpdated(record Record) {
	t.bus.Pub(msgbus.ZoneRecordUpdated{
		Name:  record.Name,
		Class: record.Class,
		Type:  record.Type,
		TTL:   record.TTL,
		Data:  record.Data,
	})
}

func (t *dns) onInstanceStatusDeleted(c msgbus.InstanceStatusDeleted) {
	name := fqdn.New(c.Path, t.cluster.Name).String()
	if records, ok := t.state[name]; ok {
		for _, record := range records {
			t.pubDeleted(record)
		}
		delete(t.state, name)
	}
}

func (t *dns) onInstanceStatusUpdated(c msgbus.InstanceStatusUpdated) {
	name := fqdn.New(c.Path, t.cluster.Name).String() + "."
	nameOnNode := fmt.Sprintf("%s.%s.%s.%s.node.%s.", c.Path.Name, c.Path.Namespace, c.Path.Kind, c.Node, t.cluster.Name)
	records := make(Zone, 0)
	updatedRecords := make(map[string]any)
	existingRecords := t.getExistingRecords(name)
	stage := func(record Record) {
		records = append(records, record)
		existingRecord, ok := existingRecords[record.Name]
		var change bool
		switch {
		case !ok:
			change = true
		case existingRecord.Data != record.Data:
			change = true
		case existingRecord.Type != record.Type:
			change = true
		case existingRecord.Class != record.Class:
			change = true
		case existingRecord.TTL != record.TTL:
			change = true
		}
		if change {
			t.pubUpdated(record)
			updatedRecords[record.Name] = nil
		}
	}
	for _, r := range c.Value.Resources {
		i, ok := r.Info[ipAddrInfoKey]
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
			aType = "A6"
			ptrType = "PTR6"
		}

		// Add a direct record (node agnostic)
		stage(Record{
			Name:  name,
			Class: "IN",
			Type:  aType,
			TTL:   60,
			Data:  ipAddr,
		})

		// Add a reverse record (node agnostic)
		stage(Record{
			Name:  reverseAddr(ip),
			Class: "IN",
			Type:  ptrType,
			TTL:   60,
			Data:  name,
		})

		// Add a direct record (node affine)
		stage(Record{
			Name:  nameOnNode,
			Class: "IN",
			Type:  aType,
			TTL:   60,
			Data:  ipAddr,
		})

		// Add a reverse record (node affine)
		stage(Record{
			Name:  reverseAddr(ip),
			Class: "IN",
			Type:  ptrType,
			TTL:   60,
			Data:  nameOnNode,
		})

		if rid, err := resourceid.Parse(r.Rid); err == nil {
			nameWithResourceName := rid.Index() + "." + name
			nameOnNodeWithResourceName := rid.Index() + "." + nameOnNode

			// Add a resource direct record (node agnostic)
			stage(Record{
				Name:  nameWithResourceName,
				Class: "IN",
				Type:  aType,
				TTL:   60,
				Data:  ipAddr,
			})

			// Add a resource reverse record (node agnostic)
			stage(Record{
				Name:  reverseAddr(ip),
				Class: "IN",
				Type:  ptrType,
				TTL:   60,
				Data:  nameWithResourceName,
			})

			// Add a direct record (node affine)
			stage(Record{
				Name:  nameOnNodeWithResourceName,
				Class: "IN",
				Type:  aType,
				TTL:   60,
				Data:  ipAddr,
			})

			// Add a reverse record (node affine)
			stage(Record{
				Name:  reverseAddr(ip),
				Class: "IN",
				Type:  ptrType,
				TTL:   60,
				Data:  nameOnNodeWithResourceName,
			})
		}
	}
	for name, record := range existingRecords {
		if _, ok := updatedRecords[name]; !ok {
			t.pubDeleted(record)
		}
	}
	if len(records) > 0 {
		t.state[name] = records
	} else {
		delete(t.state, name)
	}
}

func (t *dns) onCmdGetZone(c cmdGetZone) {
	c.resp <- t.zone()
}

func (t *dns) zone() Zone {
	zone := make(Zone, 0)
	zoneName := t.cluster.Name + "."
	for i, dns := range t.cluster.DNS {
		nsName := fmt.Sprintf("ns%d.%s", i, zoneName)
		zone = append(zone,
			Record{
				Name:  nsName,
				Class: "IN",
				Type:  "A",
				TTL:   60,
				Data:  dns,
			},
			Record{
				Name:  zoneName,
				Class: "IN",
				Type:  "NS",
				TTL:   3600,
				Data:  nsName,
			},
		)
	}
	for _, records := range t.state {
		zone = append(zone, records...)
	}
	return zone
}

func (t *dns) getExistingRecords(name string) map[string]Record {
	m := make(map[string]Record)
	records, ok := t.state[name]
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
