package arrayhds

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/util/san"
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

	// OptResizeDisk is what "resize disk" was asked for.
	OptResizeDisk struct {
		DevNum   string
		Size     string
		Truncate bool
	}

	// internalMapping is one export to make: a domain, the port it is on, and
	// the number the volume answers to there.
	internalMapping struct {
		Domain   string `json:"domain"`
		PortName string `json:"portname"`
		LUN      int    `json:"lun"`
	}

	// createdError is the error of an "add disk" failing after the array
	// created the volume. The volume is left in place, as deleting it on the
	// way out is one more change made to an array in a state nobody checked,
	// and the error names it so it can be found, mapped or deleted.
	createdError struct {
		devNum      string
		displayName string
		pool        string
		err         error
	}
)

func (t *createdError) Error() string {
	return fmt.Sprintf("volume devnum %s displayname %s was created in pool %s and is left in place: %s",
		t.devNum, t.displayName, t.pool, t.err)
}

func (t *createdError) Unwrap() error {
	return t.err
}

// AddDisk creates a volume in a pool, names it, and exports it.
//
// What is returned is what v2 returned, which a collector form stores: the
// disk id, the display name as the disk devid, the volume as the manager
// lists it, the paths to it, and the log of the commands run.
func (t *Array) AddDisk(ctx context.Context, opt OptAddDisk) (any, error) {
	if opt.Pool == "" {
		return nil, fmt.Errorf("--pool is mandatory")
	}
	if opt.Size == "" {
		return nil, fmt.Errorf("--size is mandatory")
	}
	size, err := array.ParseSize(opt.Size)
	if err != nil {
		return nil, err
	}
	if size.Relative {
		return nil, fmt.Errorf("--size %s: a new volume has no size to add to", opt.Size)
	}
	sizeKB, err := toKB(size.Bytes)
	if err != nil {
		return nil, fmt.Errorf("--size %s: %w", opt.Size, err)
	}
	pools, err := t.getPools(ctx)
	if err != nil {
		return nil, err
	}
	p, ok := poolByName(pools, opt.Pool)
	if !ok {
		return nil, fmt.Errorf("no pool named %s on this array, among the %d it lists", opt.Pool, len(pools))
	}
	if p.PoolID == "" {
		return nil, fmt.Errorf("pool %s: the array lists it with no pool id", opt.Pool)
	}

	// The mappings are resolved before the volume is created, so one the
	// array can not make fails the command before it leaves a volume behind.
	// They are resolved again when mapping, on what the array holds then.
	if len(opt.Mappings) > 0 {
		domains, ports, err := t.getDomainsAndPorts(ctx)
		if err != nil {
			return nil, err
		}
		if _, err := translateMappings(domains, ports, opt.Mappings); err != nil {
			return nil, err
		}
	}

	out, err := t.run(ctx, false, true, "addvirtualvolume",
		"capacity="+strconv.FormatInt(sizeKB, 10), "capacitytype=KB", "poolid="+p.PoolID)
	if err != nil {
		return nil, err
	}
	devNum, displayName, err := createdVolume(out)
	if err != nil {
		return nil, fmt.Errorf("addvirtualvolume in pool %s succeeded, and the array may hold a volume it created: %w", opt.Pool, err)
	}
	created := func(err error) error {
		return &createdError{devNum: devNum, displayName: displayName, pool: opt.Pool, err: err}
	}

	if opt.Name != "" {
		if _, err := t.renameDisk(ctx, devNum, opt.Name); err != nil {
			return nil, created(err)
		}
	}
	var (
		domains []hostStorageDomain
		ports   []port
	)
	if len(opt.Mappings) > 0 {
		_, domains, ports, err = t.addMap(ctx, devNum, opt.Mappings, opt.LUN)
		if err != nil {
			return nil, created(err)
		}
	}

	// The volume is read back, so what is returned says what the array holds
	// rather than what it was asked for. It is looked up by the display name
	// the array answered the creation with, as v2 looks it up, and picked by
	// its device number.
	units, err := t.getLogicalUnits(ctx, displayName)
	if err != nil {
		return nil, created(err)
	}
	unit, err := unitByDevNum(units, devNum)
	if err != nil {
		return nil, created(err)
	}
	if unit.DisplayName != displayName {
		return nil, created(fmt.Errorf("the array lists the volume with the display name %q", unit.DisplayName))
	}
	diskID := unit.diskID()
	if diskID == "" {
		return nil, created(fmt.Errorf("the object id %q of the volume holds no <serial>.<n> disk id", unit.ObjectID))
	}
	if len(unit.Paths) > 0 && (domains == nil || ports == nil) {
		if domains, ports, err = t.getDomainsAndPorts(ctx); err != nil {
			return nil, created(err)
		}
	}
	mappings, err := mappingsOf(unit, domains, ports)
	if err != nil {
		return nil, created(err)
	}
	return map[string]any{
		"disk_id":     diskID,
		"disk_devid":  unit.DisplayName,
		"mappings":    mappings,
		"driver_data": map[string]any{"lu": unit},
		"log":         t.logEntries(),
	}, nil
}

// DelDisk unexports a volume and deletes it.
func (t *Array) DelDisk(ctx context.Context, devnum string) (any, error) {
	if devnum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	devNum, err := t.devNumOf(ctx, devnum)
	if err != nil {
		return nil, err
	}
	domains, err := t.getHostStorageDomains(ctx)
	if err != nil {
		return nil, err
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("devnum %s: not deleted: the array lists no host storage domain, so the paths to remove first are unknown", devNum)
	}
	unmapped, err := t.unmap(ctx, devNum, pathsOf(domains, devNum))
	if err != nil {
		return nil, err
	}
	if _, err := t.run(ctx, false, true, "deletevirtualvolume", "devnums="+devNum); err != nil {
		return nil, fmt.Errorf("devnum %s: %s: %w", devNum, describeUnmapped(unmapped), err)
	}
	return map[string]any{
		"devnum":   devNum,
		"unmapped": unmapped,
		"log":      t.logEntries(),
	}, nil
}

// ResizeDisk resizes a volume. A size beginning with a plus is added to the
// size the volume has.
//
// The volume is read first in every case: its capacity is what a size to add
// to is added to, and what a new size is checked against, as a size below it
// drops the end of the volume and is done only when truncating is allowed.
func (t *Array) ResizeDisk(ctx context.Context, opt OptResizeDisk) (any, error) {
	if opt.DevNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	if opt.Size == "" {
		return nil, fmt.Errorf("--size is mandatory")
	}
	size, err := array.ParseSize(opt.Size)
	if err != nil {
		return nil, err
	}
	devNum, err := t.devNumOf(ctx, opt.DevNum)
	if err != nil {
		return nil, err
	}

	// The whole list is read, as v2 reads it, and the volume picked by its
	// device number: the manager matches a display name filter its own way.
	units, err := t.getLogicalUnits(ctx, "")
	if err != nil {
		return nil, err
	}
	unit, err := unitByDevNum(units, devNum)
	if err != nil {
		return nil, err
	}
	currentKB, err := unit.capacityKB()
	if err != nil {
		return nil, err
	}
	target := size.Target(currentKB * 1024)
	if err := array.CheckResize(currentKB*1024, target, opt.Truncate); err != nil {
		return nil, fmt.Errorf("devnum %s (%s): %w", devNum, unit.DisplayName, err)
	}
	targetKB, err := toKB(target)
	if err != nil {
		return nil, fmt.Errorf("devnum %s (%s): --size %s: %w", devNum, unit.DisplayName, opt.Size, err)
	}
	if targetKB == currentKB {
		t.log().Infof("devnum %s (%s) is already %d KB", devNum, unit.DisplayName, currentKB)
		return t.diskResult(unit), nil
	}

	if _, err := t.run(ctx, false, true, "modifyvirtualvolume",
		"capacity="+strconv.FormatInt(targetKB, 10), "capacitytype=KB", "devnums="+devNum); err != nil {
		return nil, fmt.Errorf("devnum %s (%s): %w", devNum, unit.DisplayName, err)
	}

	resized := func(err error) error {
		return fmt.Errorf("devnum %s (%s): modifyvirtualvolume to %d KB succeeded: %w", devNum, unit.DisplayName, targetKB, err)
	}
	units, err = t.getLogicalUnits(ctx, unit.DisplayName)
	if err != nil {
		return nil, resized(err)
	}
	after, err := unitByDevNum(units, devNum)
	if err != nil {
		return nil, resized(err)
	}
	afterKB, err := after.capacityKB()
	if err != nil {
		return nil, resized(err)
	}
	if afterKB < targetKB || (afterKB == currentKB && targetKB != currentKB) {
		return nil, resized(fmt.Errorf("the array reports %d KB, was %d KB", afterKB, currentKB))
	}
	return t.diskResult(after), nil
}

// diskResult is what an action changing a volume returns: the volume, named
// as "add disk" names it.
func (t *Array) diskResult(unit logicalUnit) map[string]any {
	return map[string]any{
		"disk_id":     unit.diskID(),
		"disk_devid":  unit.DisplayName,
		"driver_data": map[string]any{"lu": unit},
		"log":         t.logEntries(),
	}
}

// RenameDisk gives a volume a label.
func (t *Array) RenameDisk(ctx context.Context, devnum, name string) (any, error) {
	if devnum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	if name == "" {
		return nil, fmt.Errorf("--name is mandatory")
	}
	devNum, err := t.devNumOf(ctx, devnum)
	if err != nil {
		return nil, err
	}
	data, err := t.renameDisk(ctx, devNum, name)
	if err != nil {
		return nil, err
	}
	data["log"] = t.logEntries()
	return data, nil
}

// renameDisk labels a volume of a device number already resolved, and
// returns what the manager answered.
func (t *Array) renameDisk(ctx context.Context, devNum, name string) (map[string]any, error) {
	out, err := t.run(ctx, false, true, "modifylabel", "devnums="+devNum, "label="+name)
	if err != nil {
		return nil, fmt.Errorf("devnum %s: %w", devNum, err)
	}
	data := parse(out)
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	return data[0], nil
}

// AddMap exports a volume to the domains the named initiators reach it
// through.
//
// Every initiator and target pair must resolve to a port and to a domain of
// that port knowing the initiator: a pair that does not is a path the host
// would be missing, and is an error rather than skipped.
func (t *Array) AddMap(ctx context.Context, opt OptAddMap) (any, error) {
	if opt.DevNum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	if len(opt.Mappings) == 0 {
		return nil, fmt.Errorf("--mappings is mandatory")
	}
	devNum, err := t.devNumOf(ctx, opt.DevNum)
	if err != nil {
		return nil, err
	}
	paths, _, _, err := t.addMap(ctx, devNum, opt.Mappings, opt.LUN)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"devnum": devNum,
		"paths":  paths,
		"log":    t.logEntries(),
	}, nil
}

// addMap exports a volume of a device number already resolved, and returns
// the paths the manager answered, with the domains and ports it read.
func (t *Array) addMap(ctx context.Context, devNum string, mappings []string, optLUN int) ([]any, []hostStorageDomain, []port, error) {
	domains, ports, err := t.getDomainsAndPorts(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	internal, err := translateMappings(domains, ports, mappings)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("devnum %s: %w", devNum, err)
	}

	results := make([]any, 0)
	done := make([]string, 0)
	for _, mapping := range internal {
		lun := mapping.LUN
		if optLUN >= 0 {
			lun = optLUN
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
			if len(done) > 0 {
				return nil, nil, nil, fmt.Errorf("devnum %s: mapped through %s, then: %w", devNum, strings.Join(done, ", "), err)
			}
			return nil, nil, nil, fmt.Errorf("devnum %s: %w", devNum, err)
		}
		done = append(done, fmt.Sprintf("port %s domain %s lun %d", mapping.PortName, mapping.Domain, lun))
		if p := answerPath(out); p != nil {
			results = append(results, p)
		}
	}
	return results, domains, ports, nil
}

// answerPath returns the path the manager answers an export with, as v2 reads
// it, or the whole answer when it holds none.
func answerPath(out string) any {
	data := parse(out)
	if len(data) == 0 {
		return nil
	}
	if l, ok := data[0]["Path"].([]map[string]any); ok && len(l) > 0 {
		return l[0]
	}
	return data[0]
}

// DelMap unexports a volume.
//
// Naming the mappings unexports those. Naming none unexports every path to the
// volume, which is what deleting it does on the way.
func (t *Array) DelMap(ctx context.Context, devnum string, mappings []string) (any, error) {
	if devnum == "" {
		return nil, fmt.Errorf("--devnum is mandatory")
	}
	devNum, err := t.devNumOf(ctx, devnum)
	if err != nil {
		return nil, err
	}
	domains, err := t.getHostStorageDomains(ctx)
	if err != nil {
		return nil, err
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("devnum %s: the array lists no host storage domain", devNum)
	}

	var internal []internalMapping
	if len(mappings) > 0 {
		ports, err := t.getPorts(ctx)
		if err != nil {
			return nil, err
		}
		l, err := translateMappings(domains, ports, mappings)
		if err != nil {
			return nil, fmt.Errorf("devnum %s: %w", devNum, err)
		}
		for _, mapping := range l {
			if !isMapped(domains, devNum, mapping) {
				t.log().Infof("device %s is not mapped to port %s in domain %s",
					devNum, mapping.PortName, mapping.Domain)
				continue
			}
			internal = append(internal, mapping)
		}
	} else {
		internal = pathsOf(domains, devNum)
	}

	unmapped, err := t.unmap(ctx, devNum, internal)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"devnum":   devNum,
		"unmapped": unmapped,
		"log":      t.logEntries(),
	}, nil
}

// unmap removes exports of a volume, and returns those it removed.
func (t *Array) unmap(ctx context.Context, devNum string, internal []internalMapping) ([]internalMapping, error) {
	done := make([]internalMapping, 0, len(internal))
	for _, mapping := range internal {
		if _, err := t.run(ctx, false, true, "deletelun",
			"devnum="+devNum, "portname="+mapping.PortName, "domain="+mapping.Domain); err != nil {
			return nil, fmt.Errorf("devnum %s: %s: %w", devNum, describeUnmapped(done), err)
		}
		done = append(done, mapping)
	}
	return done, nil
}

// describeUnmapped says which exports were removed before an error.
func describeUnmapped(l []internalMapping) string {
	if len(l) == 0 {
		return "no path removed"
	}
	s := make([]string, len(l))
	for i, m := range l {
		s[i] = "port " + m.PortName + " domain " + m.Domain
	}
	return "paths removed: " + strings.Join(s, ", ")
}

// ListLogicalUnits returns the volumes of the array, or the one of a device
// number.
func (t *Array) ListLogicalUnits(ctx context.Context, devnum string) (any, error) {
	if devnum == "" {
		return t.getLogicalUnits(ctx, "")
	}
	devNum, err := t.devNumOf(ctx, devnum)
	if err != nil {
		return nil, err
	}
	units, err := t.getLogicalUnits(ctx, "")
	if err != nil {
		return nil, err
	}
	l := make([]logicalUnit, 0, 1)
	for _, unit := range units {
		if unit.DevNum == devNum {
			l = append(l, unit)
		}
	}
	return l, nil
}

// getDomainsAndPorts reads what a mapping is resolved with.
func (t *Array) getDomainsAndPorts(ctx context.Context) ([]hostStorageDomain, []port, error) {
	domains, err := t.getHostStorageDomains(ctx)
	if err != nil {
		return nil, nil, err
	}
	ports, err := t.getPorts(ctx)
	if err != nil {
		return nil, nil, err
	}
	return domains, ports, nil
}

// translateMappings turns the initiators and targets of a command line into
// the domains the array exports through, and the number to export as.
//
// The number is one the domains all have free, so a volume answers to the same
// number on every path to it.
//
// An empty list of ports or domains is an error, rather than nothing to map:
// it is what an answer this driver could not read looks like.
//
// A pair resolving to no port, or to no domain of its port holding the
// initiator, is skipped, as v2 skipped it: the collector names every target
// an initiator is zoned to, and a host is zoned to ports it has no domain on.
// An initiator left with no domain at all is an error though, as v2 did not
// check: the volume would be reported mapped to a host that can not see it.
func translateMappings(domains []hostStorageDomain, ports []port, mappings []string) ([]internalMapping, error) {
	if len(ports) == 0 {
		return nil, fmt.Errorf("the array lists no port")
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("the array lists no host storage domain")
	}
	paths, err := san.ParseMappings(mappings)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no mapping to make in %v", mappings)
	}

	l := make([]internalMapping, 0)
	used := make(map[int]bool)
	// initiators is the number of domains found for each initiator, and
	// skipped why the pairs of no domain were skipped.
	initiators := make(map[string]int)
	skipped := make([]string, 0)
	for _, p := range paths {
		if _, ok := initiators[p.Initiator.Name]; !ok {
			initiators[p.Initiator.Name] = 0
		}
		target, ok := portByTarget(ports, p.Target.Name)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s:%s: the array has no port %s", p.Initiator.Name, p.Target.Name, p.Target.Name))
			continue
		}
		found := domainsOf(domains, p.Initiator.Name, target.DisplayName)
		if len(found) == 0 {
			skipped = append(skipped, fmt.Sprintf("%s:%s: no host storage domain of port %s holds the initiator", p.Initiator.Name, p.Target.Name, target.DisplayName))
			continue
		}
		initiators[p.Initiator.Name] += len(found)
		for _, domain := range found {
			for _, lun := range usedLUNs(domain) {
				used[lun] = true
			}
			mapping := internalMapping{Domain: domain.DomainID, PortName: domain.PortName}
			if !hasMapping(l, mapping) {
				l = append(l, mapping)
			}
		}
	}
	for initiator, n := range initiators {
		if n == 0 {
			return nil, fmt.Errorf("initiator %s is in no host storage domain of the ports of its mappings: %s", initiator, strings.Join(skipped, "; "))
		}
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("no domain to map through for %v", mappings)
	}
	lun := freeLUN(used)
	if lun < 0 {
		return nil, fmt.Errorf("no logical unit number is free in every domain to map through")
	}
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

// mappingsOf returns the paths to a volume as the collector reads the
// mappings of a disk: indexed by "<hba_id>:<tgt_id>", each the initiator, the
// target and the logical unit number.
//
// v2 meant to return these and returned an empty map, the tables it read them
// from being left empty. A path is every initiator its domain knows on every
// world wide name of its port.
func mappingsOf(unit logicalUnit, domains []hostStorageDomain, ports []port) (map[string]any, error) {
	m := make(map[string]any)
	for _, p := range unit.Paths {
		lun, err := strconv.Atoi(p.LUN)
		if err != nil {
			return nil, fmt.Errorf("path port %s domain %s: the array reports the logical unit number %q", p.PortName, p.DomainID, p.LUN)
		}
		domain, ok := domainOf(domains, p.PortName, p.DomainID)
		if !ok {
			return nil, fmt.Errorf("path port %s domain %s: the array lists no such domain", p.PortName, p.DomainID)
		}
		target, ok := portByName(ports, p.PortName)
		if !ok || target.WWPN == "" {
			return nil, fmt.Errorf("path port %s domain %s: the array lists no world wide name for the port", p.PortName, p.DomainID)
		}
		for _, hbaID := range domain.WWNs {
			m[hbaID+":"+target.WWPN] = map[string]any{
				"hba_id": hbaID,
				"tgt_id": target.WWPN,
				"lun":    lun,
			}
		}
	}
	return m, nil
}

// toKB returns a size in bytes in the KB the manager is told a capacity in.
// A size that is not a whole number of KB is refused rather than rounded: a
// size rounded down is a volume smaller than asked for.
func toKB(b int64) (int64, error) {
	if b <= 0 {
		return 0, fmt.Errorf("a volume can not be %d bytes", b)
	}
	if b%1024 != 0 {
		return 0, fmt.Errorf("%d bytes is not a whole number of KB", b)
	}
	return b / 1024, nil
}

// createdVolume reads the device number and the display name of the volume
// the manager answered a creation with.
//
// v2 reads them at one place of the answer, the first volume of the first
// array group. They are looked for at any depth here, and the answer must
// name exactly one volume: the volume the rest of the command acts on is the
// one created, never one guessed among several.
//
// The display name must be the device number written in hexadecimal: the
// collector is handed the display name as the disk devid, and names the
// volume by it to resize or delete it, so a display name that does not read
// back to this device number would have those act on another volume.
func createdVolume(out string) (string, string, error) {
	found := make([]map[string]any, 0, 1)
	var walk func(m map[string]any)
	walk = func(m map[string]any) {
		if _, ok := m["devNum"]; ok {
			found = append(found, m)
		}
		for _, v := range m {
			if nested, ok := v.([]map[string]any); ok {
				for _, one := range nested {
					walk(one)
				}
			}
		}
	}
	for _, instance := range parse(out) {
		walk(instance)
	}
	answer := strings.TrimSpace(out)
	if len(answer) > maxOutput {
		answer = answer[:maxOutput] + "..."
	}
	// An instance nested in the volume, as its ldev, can name the device
	// number again: the volumes are told apart by their device numbers, and
	// the display name is read from the first instance naming one.
	devNums := make([]string, 0, 1)
	var volume map[string]any
	for _, m := range found {
		n := fmt.Sprint(m["devNum"])
		if !slices.Contains(devNums, n) {
			devNums = append(devNums, n)
		}
		if _, ok := m["displayName"].(string); ok && volume == nil {
			volume = m
		}
	}
	if len(devNums) != 1 {
		return "", "", fmt.Errorf("the answer names %d volumes, where one was created: %s", len(devNums), answer)
	}
	if volume == nil {
		volume = found[0]
	}
	devNum := devNums[0]
	displayName, _ := volume["displayName"].(string)
	if !isDecimal(devNum) {
		return "", "", fmt.Errorf("the answer names the device number %q, which is not a number: %s", devNum, answer)
	}
	if displayName == "" {
		return "", "", fmt.Errorf("devnum %s: the answer names no display name: %s", devNum, answer)
	}
	if n, err := toDevnum(displayName); err != nil || n != devNum {
		return "", "", fmt.Errorf("devnum %s: the display name %q the answer names does not read back to this device number: %s", devNum, displayName, answer)
	}
	return devNum, displayName, nil
}
