package arrayxtremio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/util/san"
)

type (
	// volume is one volume, as the array reported it.
	//
	// The fields acted upon are checked when the volume is read, and the
	// whole object is kept for the report, which v2 hands over as the array
	// answered it.
	volume struct {
		Name    string
		NAAName string
		Index   int

		// volSize is the size the array reports, in kilobytes. It is read
		// when a resize needs it, so a volume whose size reads oddly can
		// still be deleted.
		volSize json.RawMessage

		// raw is the whole object.
		raw map[string]any
	}

	// lunMap is one mapping of a volume to an initiator group, as the array
	// lists it.
	//
	// Index is a pointer to tell a mapping the array listed without an index
	// from the mapping of index 0: removing the one listed would otherwise
	// remove the other.
	lunMap struct {
		Index   *int   `json:"index"`
		VolName string `json:"vol-name"`
	}

	// lunMapsAnswer is the answer to a read of the mappings of a volume.
	lunMapsAnswer struct {
		LunMaps []lunMap `json:"lun-maps"`
	}

	// groupPair is one mapping to make: the volume is exported to an
	// initiator group through a target group, for the ports in Paths.
	groupPair struct {
		InitiatorGroup int
		TargetGroup    int
		Paths          []san.Path
	}
)

// MarshalJSON renders a volume as the array described it.
func (t volume) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.raw)
}

// sizeBytes returns the size of the volume in bytes.
//
// The array reports it in kilobytes, as v2 reads it, as a string or as a
// number depending on the version.
func (t volume) sizeBytes() (int64, error) {
	s := strings.Trim(string(t.volSize), `"`)
	kb, err := strconv.ParseInt(s, 10, 64)
	if err != nil || kb <= 0 {
		return 0, fmt.Errorf("volume %s: the array reports a vol-size of %s, which is not a size in kilobytes", t.Name, string(t.volSize))
	}
	return kb * 1024, nil
}

// parseVolume reads a volume object, refusing one lacking what is acted upon.
func parseVolume(b json.RawMessage) (*volume, error) {
	var typed struct {
		Name    string          `json:"name"`
		NAAName string          `json:"naa-name"`
		Index   *int            `json:"index"`
		VolSize json.RawMessage `json:"vol-size"`
	}
	if err := json.Unmarshal(b, &typed); err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	if typed.Name == "" {
		return nil, fmt.Errorf("the array described a volume with no name")
	}
	if typed.Index == nil {
		return nil, fmt.Errorf("the array described volume %s with no index", typed.Name)
	}
	return &volume{
		Name:    typed.Name,
		NAAName: typed.NAAName,
		Index:   *typed.Index,
		volSize: typed.VolSize,
		raw:     raw,
	}, nil
}

// readVolume reads the volume an answer of the array describes, from path.
func (t *Array) readVolume(ctx context.Context, path string, params map[string]string) (*volume, error) {
	if params == nil {
		params = map[string]string{}
	}
	params["full"] = "1"
	var answer struct {
		Content json.RawMessage `json:"content"`
	}
	if err := t.get(ctx, path, params, &answer); err != nil {
		return nil, err
	}
	if len(answer.Content) == 0 || string(answer.Content) == "null" {
		return nil, fmt.Errorf("GET %s: the array described no volume", path)
	}
	return parseVolume(answer.Content)
}

// GetVolume returns the volume named by index or by name.
//
// A volume named by digits is named by index, as v2 reads it. The array is
// asked and its answer is checked against what was asked for, since the
// volume is then acted upon by index: a name the array did not filter on
// would otherwise have the first volume it has deleted or resized.
//
// An index is checked not to be the name of another volume as well, which
// v2 does not check: a volume named "12" could not be told from the volume of
// index 12, and acting upon the wrong one loses data.
func (t *Array) GetVolume(ctx context.Context, ref string) (*volume, error) {
	if ref == "" {
		return nil, fmt.Errorf("--volume is mandatory")
	}
	path, params := volumePath(ref)
	v, err := t.readVolume(ctx, path, params)
	if err != nil {
		return nil, fmt.Errorf("volume %s: %w", ref, err)
	}
	index, err := strconv.Atoi(ref)
	if err != nil {
		if v.Name != ref {
			return nil, fmt.Errorf("volume %s: the array answered with volume %s (index %d)", ref, v.Name, v.Index)
		}
		return v, nil
	}
	if v.Index != index {
		return nil, fmt.Errorf("volume index %d: the array answered with volume %s (index %d)", index, v.Name, v.Index)
	}
	if err := t.checkNoVolumeNamed(ctx, ref, index); err != nil {
		return nil, err
	}
	return v, nil
}

// checkNoVolumeNamed returns an error when a volume other than the one of
// index is named name.
func (t *Array) checkNoVolumeNamed(ctx context.Context, name string, index int) error {
	var data map[string]json.RawMessage
	params := map[string]string{"full": "1", "filter": "name:eq:" + name}
	if err := t.get(ctx, "/volumes", params, &data); err != nil {
		return fmt.Errorf("volume index %d: checking no other volume is named %s: %w", index, name, err)
	}
	var list []map[string]any
	if err := json.Unmarshal(data["volumes"], &list); err != nil {
		return fmt.Errorf("volume index %d: checking no other volume is named %s: the answer holds no volumes list: %w", index, name, err)
	}
	for _, one := range list {
		if one["name"] != name {
			continue
		}
		if other, err := indexOf(one["index"]); err != nil || other != index {
			return fmt.Errorf("--volume %s names the volume of index %d, and another volume is named %s: name the volume to act upon by its name", name, index, name)
		}
	}
	return nil
}

// volumeIndexPath returns the path of the volume of an index.
func volumeIndexPath(index int) string {
	return "/volumes/" + strconv.Itoa(index)
}

// AddDisk creates a volume and exports it.
//
// Every group the volume is to be exported to is looked up before the volume
// is created, so a port the array does not know fails the command with
// nothing made. Once the volume is made, a failure is reported with the name
// and the index of the volume, which is left in place: deleting a volume is
// not something to do on the way out of an error, and the caller is told
// what to remove.
//
// What is reported is what v2 reports, which the collector stores: the
// volume as the array describes it and the mappings made, in driver_data,
// the naa of the volume as disk_id, its index as disk_devid, and the
// mappings by "<hba>:<target>". The mappings are the pairs asked for, each
// with the lun of the mapping made for it. v2 reports every pair of a port of
// the initiator group with a port of the target group, including pairs
// nobody asked for; the keys and the values are the same, there are fewer of
// them.
func (t *Array) AddDisk(ctx context.Context, opt OptAddDisk) (any, error) {
	if opt.Name == "" {
		return nil, fmt.Errorf("--name is mandatory")
	}
	if _, err := strconv.Atoi(opt.Name); err == nil {
		return nil, fmt.Errorf("--name %s: a volume named by digits can not be told from the volume of that index, which is how v2 and this driver read digits", opt.Name)
	}
	size, err := parseSize(opt.Size)
	if err != nil {
		return nil, err
	}
	if size.Relative {
		return nil, fmt.Errorf("--size %s: a new volume has no size to grow from", opt.Size)
	}
	volSize, err := mib(size.Bytes)
	if err != nil {
		return nil, err
	}
	pairs, err := t.groupPairs(ctx, opt.Mappings)
	if err != nil {
		return nil, err
	}
	data := map[string]any{
		"vol-name": opt.Name,
		"vol-size": volSize,
	}
	// An option nobody set is not sent: the array has a default for each of
	// these, and sending a zero would be choosing one.
	if opt.Blocksize > 0 {
		data["lb-size"] = opt.Blocksize
	}
	if opt.SmallIOAlerts != "" {
		data["small-io-alerts"] = opt.SmallIOAlerts
	}
	if opt.UnalignedIOAlerts != "" {
		data["unaligned-io-alerts"] = opt.UnalignedIOAlerts
	}
	if opt.Access != "" {
		data["vol-access"] = opt.Access
	}
	if opt.VAAITPAlerts != "" {
		data["vaai-tp-alerts"] = opt.VAAITPAlerts
	}
	if opt.AlignmentOffset > 0 {
		data["alignment-offset"] = opt.AlignmentOffset
	}
	href, err := t.create(ctx, "/volumes", data)
	switch {
	case errors.Is(err, errNoAnswer):
		return nil, fmt.Errorf("volume %s may have been created, the array did not answer its creation: check with list volumes --volume %s: %w", opt.Name, opt.Name, err)
	case errors.Is(err, errCreatedUnnamed):
		return nil, fmt.Errorf("volume %s was created and is left in place, unmapped: check with list volumes --volume %s: %w", opt.Name, opt.Name, err)
	case err != nil:
		return nil, err
	}
	created, err := t.readVolume(ctx, href, nil)
	if err != nil {
		return nil, fmt.Errorf("volume %s was created and is left in place, reading it back from %s failed: %w", opt.Name, href, err)
	}
	if created.Name != opt.Name {
		return nil, fmt.Errorf("volume %s was created and is left in place, and the array points at volume %s (index %d) as the one created", opt.Name, created.Name, created.Index)
	}
	createdErr := func(made int, err error) error {
		return fmt.Errorf("volume %s (index %d) was created and is left in place, with %d of its %d mappings made: %w", created.Name, created.Index, made, len(pairs), err)
	}

	// The mappings are made with the index of the volume rather than its
	// name, and so are the reads that follow, since that is what the array
	// answered with.
	made := make([]any, 0, len(pairs))
	mappings := make(map[string]any)
	for i, pair := range pairs {
		content, err := t.addMap(ctx, created.Index, pair.InitiatorGroup, pair.TargetGroup, -1)
		if err != nil {
			return nil, createdErr(i, err)
		}
		lun, err := indexOf(content["lun"])
		if err != nil {
			return nil, createdErr(i+1, fmt.Errorf("the mapping to initiator group %d and target group %d has no lun: %w", pair.InitiatorGroup, pair.TargetGroup, err))
		}
		made = append(made, content)
		for _, p := range pair.Paths {
			mappings[p.Initiator.Name+":"+p.Target.Name] = map[string]any{
				"hba_id": p.Initiator.Name,
				"tgt_id": p.Target.Name,
				"lun":    lun,
			}
		}
	}
	v, err := t.readVolume(ctx, volumeIndexPath(created.Index), nil)
	if err != nil {
		return nil, createdErr(len(pairs), err)
	}
	if v.Name != created.Name || v.Index != created.Index {
		return nil, createdErr(len(pairs), fmt.Errorf("reading it back by index, the array answered with volume %s (index %d)", v.Name, v.Index))
	}
	if v.NAAName == "" {
		return nil, createdErr(len(pairs), fmt.Errorf("the array reports no naa-name, which is the disk id"))
	}
	return map[string]any{
		"driver_data": map[string]any{
			"volume":   v,
			"mappings": made,
		},
		"disk_id":    v.NAAName,
		"disk_devid": v.Index,
		"mappings":   mappings,
	}, nil
}

// groupPairs returns the mappings to make for the ports a command line names:
// one per pair of an initiator group and a target group, in a stable order,
// so two runs of one command make the same mappings in the same order.
//
// This is the set of mappings v2 makes, each group pair once.
func (t *Array) groupPairs(ctx context.Context, mappings []string) ([]groupPair, error) {
	paths, err := parseMappings(mappings)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		// A volume made and exported nowhere is no disk for anyone, and
		// the success saying so would hide that the mappings were empty.
		return nil, fmt.Errorf("no mapping: --mappings names no <hba>:<tgt> path")
	}
	igs := make(map[string]int)
	tgs := make(map[string]int)
	byGroups := make(map[[2]int]*groupPair)
	for _, p := range paths {
		ig, ok := igs[p.Initiator.Name]
		if !ok {
			if ig, err = t.initiatorGroupOf(ctx, p.Initiator.Name); err != nil {
				return nil, err
			}
			igs[p.Initiator.Name] = ig
		}
		tg, ok := tgs[p.Target.Name]
		if !ok {
			if tg, err = t.targetGroupOf(ctx, p.Target.Name); err != nil {
				return nil, err
			}
			tgs[p.Target.Name] = tg
		}
		key := [2]int{ig, tg}
		pair, ok := byGroups[key]
		if !ok {
			pair = &groupPair{InitiatorGroup: ig, TargetGroup: tg}
			byGroups[key] = pair
		}
		pair.Paths = append(pair.Paths, p)
	}
	l := make([]groupPair, 0, len(byGroups))
	for _, pair := range byGroups {
		l = append(l, *pair)
	}
	sort.Slice(l, func(i, j int) bool {
		if l[i].InitiatorGroup != l[j].InitiatorGroup {
			return l[i].InitiatorGroup < l[j].InitiatorGroup
		}
		return l[i].TargetGroup < l[j].TargetGroup
	})
	return l, nil
}

// DelDisk unexports a volume and deletes it.
//
// The volume is read first, and checked to be the one named, then acted upon
// by its index.
func (t *Array) DelDisk(ctx context.Context, ref string) (any, error) {
	v, err := t.GetVolume(ctx, ref)
	if err != nil {
		return nil, err
	}
	if err := t.delMappingsOf(ctx, v); err != nil {
		return nil, err
	}
	if err := t.del(ctx, volumeIndexPath(v.Index), nil); err != nil {
		return nil, fmt.Errorf("volume %s (index %d) is unexported: %w", v.Name, v.Index, err)
	}
	return map[string]any{"disk_id": v.NAAName}, nil
}

// ResizeDisk resizes a volume. A size beginning with a plus is added to the
// size the volume has.
//
// The size the volume has is read in either case, because a size below it
// drops the end of the volume, which is refused unless truncate allows it.
func (t *Array) ResizeDisk(ctx context.Context, ref, sizeExpr string, truncate bool) (any, error) {
	if ref == "" {
		return nil, fmt.Errorf("--volume is mandatory")
	}
	size, err := parseSize(sizeExpr)
	if err != nil {
		return nil, err
	}
	v, err := t.GetVolume(ctx, ref)
	if err != nil {
		return nil, err
	}
	current, err := v.sizeBytes()
	if err != nil {
		return nil, err
	}
	// A volume made elsewhere may not be a whole number of megabytes, and
	// the array is told one: the target is rounded up, which keeps the end
	// of the volume.
	const mb = 1024 * 1024
	target := (size.Target(current) + mb - 1) / mb * mb
	if err := array.CheckResize(current, target, truncate); err != nil {
		return nil, fmt.Errorf("volume %s (index %d): %w", v.Name, v.Index, err)
	}
	if target == current {
		return v, nil
	}
	volSize, err := mib(target)
	if err != nil {
		return nil, err
	}
	path := volumeIndexPath(v.Index)
	if err := t.put(ctx, path, nil, map[string]any{"vol-size": volSize}); err != nil {
		return nil, fmt.Errorf("volume %s (index %d): %w", v.Name, v.Index, err)
	}
	after, err := t.readVolume(ctx, path, nil)
	if err != nil {
		return nil, fmt.Errorf("volume %s (index %d) resized, reading it back: %w", v.Name, v.Index, err)
	}
	if after.Name != v.Name {
		return nil, fmt.Errorf("volume %s (index %d) resized, reading it back the array answered with volume %s", v.Name, v.Index, after.Name)
	}
	got, err := after.sizeBytes()
	if err != nil {
		return nil, err
	}
	if got != target {
		// The target is a whole number of MiB, which the array keeps as
		// given: any other size is a resize it did not do, a shrink it
		// ignored as much as a growth it cut short.
		return nil, fmt.Errorf("volume %s (index %d) resized to %d bytes, and the array reports %d bytes", v.Name, v.Index, target, got)
	}
	return after, nil
}

// AddMap exports a volume.
//
// Naming the mappings asks the array which groups the ports belong to, and one
// mapping is made per pair of groups. Naming an initiator group instead makes
// the one mapping it names.
//
// The volume is read first, and checked to be the one named, then mapped by
// its index.
func (t *Array) AddMap(ctx context.Context, opt OptAddMap) (any, error) {
	if len(opt.Mappings) == 0 && opt.InitiatorGroup == "" {
		return nil, fmt.Errorf("--initiatorgroup or --mappings is mandatory")
	}
	var pairs []groupPair
	if len(opt.Mappings) > 0 && opt.InitiatorGroup == "" {
		var err error
		if pairs, err = t.groupPairs(ctx, opt.Mappings); err != nil {
			return nil, err
		}
	}
	v, err := t.GetVolume(ctx, opt.Volume)
	if err != nil {
		return nil, err
	}
	results := make([]any, 0)
	if pairs == nil {
		one, err := t.addMap(ctx, v.Index, opt.InitiatorGroup, opt.TargetGroup, opt.LUN)
		if err != nil {
			return nil, err
		}
		return append(results, one), nil
	}
	for i, pair := range pairs {
		one, err := t.addMap(ctx, v.Index, pair.InitiatorGroup, pair.TargetGroup, opt.LUN)
		if err != nil {
			return nil, fmt.Errorf("volume %s (index %d): %d of %d mappings made: %w", v.Name, v.Index, i, len(pairs), err)
		}
		results = append(results, one)
	}
	return results, nil
}

// addMap makes one mapping of the volume of an index.
//
// A group is given as an index or as what a command line names it with, and
// a target group left empty is not sent, so the array picks its default.
func (t *Array) addMap(ctx context.Context, volIndex int, initiatorGroup, targetGroup any, lun int) (map[string]any, error) {
	if initiatorGroup == nil || initiatorGroup == "" {
		return nil, fmt.Errorf("--initiatorgroup is mandatory")
	}
	data := map[string]any{
		"vol-id": volIndex,
		"ig-id":  initiatorGroup,
	}
	if targetGroup != nil && targetGroup != "" {
		data["tg-id"] = targetGroup
	}
	if lun >= 0 {
		data["lun"] = lun
	}
	var answer struct {
		Content map[string]any `json:"content"`
	}
	if err := t.post(ctx, "/lun-maps", data, &answer); err != nil {
		return nil, err
	}
	if answer.Content == nil {
		return nil, fmt.Errorf("POST /lun-maps: the array described no mapping")
	}
	return answer.Content, nil
}

// DelMap removes one mapping, named by index or by name.
func (t *Array) DelMap(ctx context.Context, mapping string) error {
	if mapping == "" {
		return fmt.Errorf("--mapping is mandatory")
	}
	path := "/lun-maps"
	params := map[string]string{}
	if _, err := strconv.Atoi(mapping); err == nil {
		path += "/" + mapping
	} else {
		params["name"] = mapping
	}
	return t.del(ctx, path, params)
}

// volumeMappings returns the indexes of the mappings of a volume.
//
// The array is asked for them by a filter on the volume name, and the answer
// is checked rather than trusted: a filter the array did not apply would hand
// back the mappings of other volumes, and removing them unexports a disk in
// use. A mapping listed without the name of its volume, or without an index,
// fails the whole read before anything is removed.
func (t *Array) volumeMappings(ctx context.Context, v *volume) ([]int, error) {
	var answer lunMapsAnswer
	params := map[string]string{"full": "1", "filter": "vol-name:eq:" + v.Name}
	if err := t.get(ctx, "/lun-maps", params, &answer); err != nil {
		return nil, err
	}
	l := make([]int, 0, len(answer.LunMaps))
	for i, m := range answer.LunMaps {
		switch {
		case m.VolName == "":
			return nil, fmt.Errorf("volume %s: the array lists a mapping with no vol-name, which can not be told to be of this volume", v.Name)
		case m.VolName != v.Name:
			continue
		case m.Index == nil:
			return nil, fmt.Errorf("volume %s: the array lists mapping #%d with no index", v.Name, i)
		}
		l = append(l, *m.Index)
	}
	return l, nil
}

// delMappingsOf unexports a volume from everything it is exported to.
func (t *Array) delMappingsOf(ctx context.Context, v *volume) error {
	indexes, err := t.volumeMappings(ctx, v)
	if err != nil {
		return err
	}
	for i, index := range indexes {
		if err := t.DelMap(ctx, strconv.Itoa(index)); err != nil {
			return fmt.Errorf("volume %s (index %d): %d of %d mappings removed: %w", v.Name, v.Index, i, len(indexes), err)
		}
	}
	return nil
}

// DelVolumeMappings unexports the volume named by index or by name from
// everything it is exported to.
func (t *Array) DelVolumeMappings(ctx context.Context, ref string) error {
	v, err := t.GetVolume(ctx, ref)
	if err != nil {
		return err
	}
	return t.delMappingsOf(ctx, v)
}
