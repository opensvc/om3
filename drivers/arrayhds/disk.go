package arrayhds

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// OptAddDisk is what "add disk" was asked for.
	OptAddDisk struct {
		Name     string
		Pool     string
		Size     string
		LUN      int
		Mappings []string
	}

	// OptAddMap is what "add map" was asked for.
	OptAddMap struct {
		DevNum   string
		Mappings []string
		LUN      int
	}

	// internalMapping is one export to make: a domain, the port it is on, and
	// the number the volume answers to there.
	internalMapping struct {
		Domain   string `json:"domain"`
		PortName string `json:"portname"`
		LUN      int    `json:"lun"`
	}
)

// AddDisk creates a volume in a pool, names it, and exports it.
func (t *Array) AddDisk(ctx context.Context, opt OptAddDisk) (any, error) {
	if opt.Pool == "" {
		return nil, fmt.Errorf("--pool is mandatory")
	}
	if opt.Size == "" {
		return nil, fmt.Errorf("--size is mandatory")
	}
	sizeKB, err := sizeKB(opt.Size)
	if err != nil {
		return nil, err
	}
	pools, err := t.getPools(ctx)
	if err != nil {
		return nil, err
	}
	p, ok := poolByName(pools, opt.Pool)
	if !ok {
		return nil, fmt.Errorf("no pool named %s on this array", opt.Pool)
	}

	out, err := t.run(ctx, false, true, "addvirtualvolume",
		"capacity="+sizeKB, "capacitytype=KB", "poolid="+p.PoolID)
	if err != nil {
		return nil, err
	}
	devNum, err := devNumOfAnswer(out)
	if err != nil {
		return nil, err
	}

	if opt.Name != "" {
		if _, err := t.RenameDisk(ctx, devNum, opt.Name); err != nil {
			return nil, err
		}
	}
	if len(opt.Mappings) > 0 {
		if _, err := t.AddMap(ctx, OptAddMap{DevNum: devNum, Mappings: opt.Mappings, LUN: opt.LUN}); err != nil {
			return nil, err
		}
	}

	// The volume is read back, so what is returned says what the array holds
	// rather than what it was asked for.
	units, err := t.getLogicalUnits(ctx, devNum)
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, fmt.Errorf("the array reports no volume %s after creating it", devNum)
	}
	unit := units[0]

	mappings := make(map[string]any)
	for _, path := range unit.Paths {
		mappings[path.PortName+":"+path.LUN] = map[string]any{
			"domain":   path.DomainID,
			"portname": path.PortName,
			"lun":      path.LUN,
		}
	}
	return map[string]any{
		"driver_data": map[string]any{"lu": unit},
		"disk_id":     t.serial() + "." + unit.DevNum,
		"disk_devid":  unit.DevNum,
		"mappings":    mappings,
	}, nil
}

// DelDisk unexports a volume and deletes it.
func (t *Array) DelDisk(ctx context.Context, devNum string) (any, error) {
	if devNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	devNum = toDevnum(devNum)
	if _, err := t.DelMap(ctx, devNum, nil); err != nil {
		return nil, err
	}
	if _, err := t.run(ctx, false, true, "deletevirtualvolume", "devnums="+devNum); err != nil {
		return nil, err
	}
	return map[string]any{"disk_id": t.serial() + "." + devNum, "disk_devid": devNum}, nil
}

// ResizeDisk grows a volume. A size beginning with a plus is added to the size
// the volume has.
func (t *Array) ResizeDisk(ctx context.Context, devNum, size string) (any, error) {
	if devNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	if size == "" {
		return nil, fmt.Errorf("--size is mandatory")
	}
	devNum = toDevnum(devNum)

	var capacity string
	if strings.HasPrefix(size, "+") {
		units, err := t.getLogicalUnits(ctx, devNum)
		if err != nil {
			return nil, err
		}
		if len(units) == 0 {
			return nil, fmt.Errorf("the array reports no volume %s", devNum)
		}
		current, err := strconv.ParseInt(units[0].CapacityInKB, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("the array reports a capacity of %q: %w", units[0].CapacityInKB, err)
		}
		incr, err := sizeconv.FromSize(strings.TrimPrefix(size, "+"))
		if err != nil {
			return nil, err
		}
		capacity = strconv.FormatInt(current+incr/1024, 10)
	} else {
		s, err := sizeKB(size)
		if err != nil {
			return nil, err
		}
		capacity = s
	}

	if _, err := t.run(ctx, false, true, "modifyvirtualvolume",
		"capacity="+capacity, "capacitytype=KB", "devnums="+devNum); err != nil {
		return nil, err
	}
	units, err := t.getLogicalUnits(ctx, devNum)
	if err != nil {
		return nil, err
	}
	if len(units) == 0 {
		return nil, nil
	}
	return units[0], nil
}

// RenameDisk gives a volume a label.
func (t *Array) RenameDisk(ctx context.Context, devNum, name string) (any, error) {
	if devNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	if name == "" {
		return nil, fmt.Errorf("--name is mandatory")
	}
	out, err := t.run(ctx, false, true, "modifylabel", "devnums="+toDevnum(devNum), "label="+name)
	if err != nil {
		return nil, err
	}
	data := parse(out)
	if len(data) == 0 {
		return nil, nil
	}
	return data[0], nil
}

// AddMap exports a volume to the domains the named initiators reach it
// through.
func (t *Array) AddMap(ctx context.Context, opt OptAddMap) (any, error) {
	if opt.DevNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	if len(opt.Mappings) == 0 {
		return nil, fmt.Errorf("--mappings is mandatory")
	}
	devNum := toDevnum(opt.DevNum)
	mappings, err := t.translateMappings(ctx, opt.Mappings)
	if err != nil {
		return nil, err
	}
	domains, err := t.getHostStorageDomains(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]any, 0)
	for _, mapping := range mappings {
		lun := mapping.LUN
		if opt.LUN >= 0 {
			lun = opt.LUN
		}
		if isMapped(domains, devNum, mapping) {
			t.log().Infof("device %s is already mapped to port %s in domain %s",
				devNum, mapping.PortName, mapping.Domain)
			continue
		}
		out, err := t.run(ctx, false, true, "addlun",
			"devnum="+devNum, "portname="+mapping.PortName,
			"domain="+mapping.Domain, "lun="+strconv.Itoa(lun))
		if err != nil {
			return nil, err
		}
		if data := parse(out); len(data) > 0 {
			results = append(results, data[0])
		}
	}
	return results, nil
}

// DelMap unexports a volume.
//
// Naming the mappings unexports those. Naming none unexports every path to the
// volume, which is what deleting it does on the way.
func (t *Array) DelMap(ctx context.Context, devNum string, mappings []string) (any, error) {
	if devNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	devNum = toDevnum(devNum)

	var internal []internalMapping
	if len(mappings) > 0 {
		l, err := t.translateMappings(ctx, mappings)
		if err != nil {
			return nil, err
		}
		internal = l
	} else {
		domains, err := t.getHostStorageDomains(ctx)
		if err != nil {
			return nil, err
		}
		internal = pathsOf(domains, devNum)
	}

	for _, mapping := range internal {
		if _, err := t.run(ctx, false, true, "deletelun",
			"devnum="+devNum, "portname="+mapping.PortName, "domain="+mapping.Domain); err != nil {
			return nil, err
		}
	}
	return internal, nil
}

// ListLogicalUnits returns the volumes of the array.
func (t *Array) ListLogicalUnits(ctx context.Context, devNum string) (any, error) {
	if devNum != "" {
		devNum = toDevnum(devNum)
	}
	return t.getLogicalUnits(ctx, devNum)
}

// translateMappings turns the initiators and targets of a command line into
// the domains the array exports through, and the number to export as.
//
// The number is one the domains all have free, so a volume answers to the same
// number on every path to it.
func (t *Array) translateMappings(ctx context.Context, mappings []string) ([]internalMapping, error) {
	paths, err := san.ParseMappings(mappings)
	if err != nil {
		return nil, err
	}
	domains, err := t.getHostStorageDomains(ctx)
	if err != nil {
		return nil, err
	}
	ports, err := t.getPorts(ctx)
	if err != nil {
		return nil, err
	}

	l := make([]internalMapping, 0)
	used := make(map[int]bool)
	for _, p := range paths {
		target, ok := portByTarget(ports, p.Target.Name)
		if !ok {
			continue
		}
		for _, domain := range domainsOf(domains, p.Initiator.Name, target.DisplayName) {
			for _, lun := range usedLUNs(domain) {
				used[lun] = true
			}
			mapping := internalMapping{Domain: domain.DomainID, PortName: domain.PortName}
			if !hasMapping(l, mapping) {
				l = append(l, mapping)
			}
		}
	}
	lun := freeLUN(used)
	for i := range l {
		l[i].LUN = lun
	}
	return l, nil
}

// hasMapping reports whether an export is already in a list.
func hasMapping(l []internalMapping, m internalMapping) bool {
	for _, one := range l {
		if one.Domain == m.Domain && one.PortName == m.PortName {
			return true
		}
	}
	return false
}

// isMapped reports whether a volume is already exported through a domain.
func isMapped(domains []hostStorageDomain, devNum string, m internalMapping) bool {
	for _, domain := range domains {
		if domain.DomainID != m.Domain || domain.PortName != m.PortName {
			continue
		}
		for _, p := range domain.Paths {
			if p.DevNum == devNum {
				return true
			}
		}
	}
	return false
}

// pathsOf returns every export of a volume.
func pathsOf(domains []hostStorageDomain, devNum string) []internalMapping {
	l := make([]internalMapping, 0)
	for _, domain := range domains {
		for _, p := range domain.Paths {
			if p.DevNum != devNum {
				continue
			}
			mapping := internalMapping{Domain: p.DomainID, PortName: p.PortName}
			if !hasMapping(l, mapping) {
				l = append(l, mapping)
			}
		}
	}
	return l
}

// sizeKB renders a size expression the way the manager reads one.
func sizeKB(size string) (string, error) {
	b, err := sizeconv.FromSize(size)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(b/1024, 10), nil
}

// devNumOfAnswer reads the device number out of what the manager answered a
// creation with.
func devNumOfAnswer(out string) (string, error) {
	for _, instance := range parse(out) {
		if devNum, ok := findDevNum(instance); ok {
			return devNum, nil
		}
	}
	return "", fmt.Errorf("the array named no device in its answer: %s", strings.TrimSpace(out))
}

// findDevNum looks for a device number in an answer, at any depth.
func findDevNum(m map[string]any) (string, bool) {
	if v, ok := m["devNum"]; ok {
		return fmt.Sprint(v), true
	}
	for _, v := range m {
		nested, ok := v.([]map[string]any)
		if !ok {
			continue
		}
		for _, one := range nested {
			if devNum, ok := findDevNum(one); ok {
				return devNum, true
			}
		}
	}
	return "", false
}
