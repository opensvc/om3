package arrayhds

import (
	"context"
	"encoding/xml"
	"strconv"
	"strings"
)

type (
	// storageArray is what the manager answers a query with.
	storageArray struct {
		XMLName            xml.Name            `xml:"Response"`
		LogicalUnits       []logicalUnit       `xml:"ResponseData>StorageArray>LogicalUnit"`
		HostStorageDomains []hostStorageDomain `xml:"ResponseData>StorageArray>HostStorageDomain"`
		Pools              []pool              `xml:"ResponseData>StorageArray>Pool"`
		Ports              []port              `xml:"ResponseData>StorageArray>Port"`
	}

	// logicalUnit is one volume of the array.
	logicalUnit struct {
		ObjectID     string `xml:"objectID,attr"`
		DevNum       string `xml:"devNum,attr"`
		DisplayName  string `xml:"displayName,attr"`
		CapacityInKB string `xml:"capacityInKB,attr"`
		Name         string `xml:"name,attr"`
		Paths        []path `xml:"Path"`
	}

	// hostStorageDomain is one domain a volume is exported through.
	hostStorageDomain struct {
		ObjectID string `xml:"objectID,attr"`
		DomainID string `xml:"domainID,attr"`
		PortName string `xml:"portName,attr"`
		WWNs     []wwn  `xml:"WWN"`
		Paths    []path `xml:"Path"`
	}

	// wwn is one initiator known to a domain.
	wwn struct {
		WWN string `xml:"WWN,attr"`
	}

	// path is one export of a volume.
	path struct {
		DevNum   string `xml:"devNum,attr"`
		DomainID string `xml:"domainID,attr"`
		PortName string `xml:"portName,attr"`
		LUN      string `xml:"LUN,attr"`
	}

	// pool is one pool a volume can be carved from.
	pool struct {
		PoolID string `xml:"poolID,attr"`
		Name   string `xml:"name,attr"`
	}

	// port is one port of the array.
	port struct {
		ObjectID    string `xml:"objectID,attr"`
		DisplayName string `xml:"displayName,attr"`
		WWN         string `xml:"wwn,attr"`
	}
)

// normalizedWWN returns a world wide name the way the array stores one, so a
// name written with dots and in upper case matches one written without.
func normalizedWWN(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, ".", ""))
}

// getLogicalUnits returns the volumes of the array, with their exports.
//
// Naming one reads that one, which is what an action does after changing it:
// the manager answers a change with the device number and little else.
func (t *Array) getLogicalUnits(ctx context.Context, displayName string) ([]logicalUnit, error) {
	args := []string{"GetStorageArray", "subtarget=Logicalunit", "lusubinfo=Path,LDEV,VolumeConnection"}
	if displayName != "" {
		args = append(args, "displayname="+displayName)
	}
	out, err := t.run(ctx, true, true, args...)
	if err != nil {
		return nil, err
	}
	var data storageArray
	if err := unmarshalXML(out, &data); err != nil {
		return nil, err
	}
	return data.LogicalUnits, nil
}

// getHostStorageDomains returns the domains of the array, with the initiators
// they know and the volumes they export.
func (t *Array) getHostStorageDomains(ctx context.Context) ([]hostStorageDomain, error) {
	out, err := t.run(ctx, true, true, "GetStorageArray", "subtarget=HostStorageDomain", "hsdsubinfo=WWN,Path")
	if err != nil {
		return nil, err
	}
	var data storageArray
	if err := unmarshalXML(out, &data); err != nil {
		return nil, err
	}
	for i, domain := range data.HostStorageDomains {
		for j, w := range domain.WWNs {
			data.HostStorageDomains[i].WWNs[j].WWN = normalizedWWN(w.WWN)
		}
	}
	return data.HostStorageDomains, nil
}

// getPools returns the pools of the array.
func (t *Array) getPools(ctx context.Context) ([]pool, error) {
	out, err := t.run(ctx, true, true, "GetStorageArray", "subtarget=Pool")
	if err != nil {
		return nil, err
	}
	var data storageArray
	if err := unmarshalXML(out, &data); err != nil {
		return nil, err
	}
	return data.Pools, nil
}

// getPorts returns the ports of the array.
func (t *Array) getPorts(ctx context.Context) ([]port, error) {
	out, err := t.run(ctx, true, true, "GetStorageArray", "subtarget=Port")
	if err != nil {
		return nil, err
	}
	var data storageArray
	if err := unmarshalXML(out, &data); err != nil {
		return nil, err
	}
	return data.Ports, nil
}

// poolByName returns the pool a name refers to.
func poolByName(pools []pool, name string) (pool, bool) {
	for _, p := range pools {
		if p.Name == name {
			return p, true
		}
	}
	return pool{}, false
}

// portByTarget returns the port a target is reached through.
func portByTarget(ports []port, target string) (port, bool) {
	target = normalizedWWN(target)
	for _, p := range ports {
		if normalizedWWN(p.WWN) == target || strings.EqualFold(p.DisplayName, target) {
			return p, true
		}
	}
	return port{}, false
}

// domainsOf returns the domains an initiator reaches a port through.
func domainsOf(domains []hostStorageDomain, hbaID string, portName string) []hostStorageDomain {
	hbaID = normalizedWWN(hbaID)
	l := make([]hostStorageDomain, 0)
	for _, domain := range domains {
		if domain.PortName != portName {
			continue
		}
		for _, w := range domain.WWNs {
			if w.WWN == hbaID {
				l = append(l, domain)
				break
			}
		}
	}
	return l
}

// usedLUNs returns the logical unit numbers a domain already hands out.
func usedLUNs(domain hostStorageDomain) []int {
	l := make([]int, 0, len(domain.Paths))
	for _, p := range domain.Paths {
		if i, err := strconv.Atoi(p.LUN); err == nil {
			l = append(l, i)
		}
	}
	return l
}

// freeLUN returns the lowest logical unit number none of the domains hands
// out, so one volume is the same number on every path to it.
func freeLUN(used map[int]bool) int {
	for lun := 0; lun < 65536; lun++ {
		if !used[lun] {
			return lun
		}
	}
	return -1
}
