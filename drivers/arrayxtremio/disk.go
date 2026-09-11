package arrayxtremio

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/san"
)

type (
	// volumeContent is what the array says about one volume.
	volumeContent struct {
		Name    string `json:"name"`
		NAAName string `json:"naa-name"`
		Index   int    `json:"index"`
		VolSize string `json:"vol-size"`
	}

	// volumeAnswer is the answer to a read naming one volume.
	volumeAnswer struct {
		Content *volumeContent `json:"content"`
	}

	// lunMap is one mapping of a volume to an initiator group.
	lunMap struct {
		Index int `json:"index"`
	}

	// lunMapsAnswer is the answer to a read of the mappings of a volume.
	lunMapsAnswer struct {
		LunMaps []lunMap `json:"lun-maps"`
	}
)

// GetVolume returns what the array says about one volume, named by index or by
// name.
func (t *Array) GetVolume(ctx context.Context, volume string) (*volumeContent, error) {
	if volume == "" {
		return nil, fmt.Errorf("--volume is mandatory")
	}
	path, params := volumePath(volume)
	params["full"] = "1"
	var answer volumeAnswer
	if err := t.get(ctx, path, params, &answer); err != nil {
		return nil, err
	}
	if answer.Content == nil {
		return nil, fmt.Errorf("volume %s does not exist", volume)
	}
	return answer.Content, nil
}

// AddDisk creates a volume and exports it.
func (t *Array) AddDisk(ctx context.Context, opt OptAddDisk) (any, error) {
	if opt.Name == "" {
		return nil, fmt.Errorf("--name is mandatory")
	}
	if opt.Size == "" {
		return nil, fmt.Errorf("--size is mandatory")
	}
	size, err := sizeMB(opt.Size)
	if err != nil {
		return nil, err
	}
	data := map[string]any{
		"vol-name": opt.Name,
		"vol-size": size,
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
	if err := t.post(ctx, "/volumes", data, nil); err != nil {
		return nil, err
	}

	driverData := map[string]any{}
	made := make([]any, 0)
	paths := make(map[string][]san.Path)
	if len(opt.Mappings) > 0 {
		var err error
		made, paths, err = t.addMappings(ctx, opt.Name, opt.Mappings, -1)
		if err != nil {
			return nil, err
		}
	}
	volume, err := t.GetVolume(ctx, opt.Name)
	if err != nil {
		return nil, err
	}
	driverData["volume"] = volume
	driverData["mappings"] = made

	// The mappings are reported by the ports they were asked for rather than
	// by the groups the array made them between: the caller named ports, and
	// the collector accounts for the disk by them.
	mappings := make(map[string]any)
	for key, l := range paths {
		lun := lunOf(made, key)
		for _, p := range l {
			mappings[p.Initiator.Name+":"+p.Target.Name] = map[string]any{
				"hba_id": p.Initiator.Name,
				"tgt_id": p.Target.Name,
				"lun":    lun,
			}
		}
	}

	return map[string]any{
		"driver_data": driverData,
		"disk_id":     volume.NAAName,
		"disk_devid":  volume.Index,
		"mappings":    mappings,
	}, nil
}

// addMappings exports a volume to the groups the named ports belong to, and
// returns what was made along with the ports each group pair stands for.
func (t *Array) addMappings(ctx context.Context, volume string, mappings []string, lun int) ([]any, map[string][]san.Path, error) {
	made := make([]any, 0)
	paths, err := parseMappings(mappings)
	if err != nil {
		return nil, nil, err
	}
	byGroups := make(map[string][]san.Path)
	for _, p := range paths {
		ig, err := t.initiatorGroupOf(ctx, p.Initiator.Name)
		if err != nil {
			return nil, nil, err
		}
		tg, err := t.targetGroupOf(ctx, p.Target.Name)
		if err != nil {
			return nil, nil, err
		}
		byGroups[ig+":"+tg] = append(byGroups[ig+":"+tg], p)
	}
	for _, key := range sortedPathKeys(byGroups) {
		ig, tg, _ := strings.Cut(key, ":")
		one, err := t.addMap(ctx, volume, ig, tg, lun)
		if err != nil {
			return nil, nil, err
		}
		made = append(made, map[string]any{"groups": key, "content": one})
	}
	return made, byGroups, nil
}

// lunOf returns the lun the array gave a mapping.
func lunOf(made []any, groups string) any {
	for _, one := range made {
		m, ok := one.(map[string]any)
		if !ok || m["groups"] != groups {
			continue
		}
		content, ok := m["content"].(map[string]any)
		if !ok {
			return nil
		}
		return content["lun"]
	}
	return nil
}

// DelDisk unexports a volume and deletes it.
func (t *Array) DelDisk(ctx context.Context, volume string) (any, error) {
	if volume == "" {
		return nil, fmt.Errorf("--volume is mandatory")
	}
	content, err := t.GetVolume(ctx, volume)
	if err != nil {
		return nil, err
	}
	if err := t.DelVolumeMappings(ctx, volume); err != nil {
		return nil, err
	}
	path, params := volumePath(volume)
	if err := t.del(ctx, path, params); err != nil {
		return nil, err
	}
	return map[string]any{"disk_id": content.NAAName}, nil
}

// ResizeDisk grows a volume. A size beginning with a plus is added to the
// size the volume has.
func (t *Array) ResizeDisk(ctx context.Context, volume, size string) (any, error) {
	if volume == "" {
		return nil, fmt.Errorf("--volume is mandatory")
	}
	if size == "" {
		return nil, fmt.Errorf("--size is mandatory")
	}
	if len(size) > 0 && size[0] == '+' {
		content, err := t.GetVolume(ctx, volume)
		if err != nil {
			return nil, err
		}
		current, err := strconv.ParseInt(content.VolSize, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("the array reports a volume size of %q: %w", content.VolSize, err)
		}
		incr, err := sizeBytes(size[1:])
		if err != nil {
			return nil, err
		}
		// The array reports a size in kilobytes.
		size = fmt.Sprintf("%dk", current+incr/1024)
	}
	mb, err := sizeMB(size)
	if err != nil {
		return nil, err
	}
	path, params := volumePath(volume)
	if err := t.put(ctx, path, params, map[string]any{"vol-size": mb}); err != nil {
		return nil, err
	}
	return t.GetVolume(ctx, volume)
}

// AddMap exports a volume.
//
// Naming the mappings asks the array which groups the ports belong to, and one
// mapping is made per pair of groups. Naming an initiator group instead makes
// the one mapping it names.
func (t *Array) AddMap(ctx context.Context, opt OptAddMap) (any, error) {
	if opt.Volume == "" {
		return nil, fmt.Errorf("--volume is mandatory")
	}
	results := make([]any, 0)
	if len(opt.Mappings) > 0 && opt.InitiatorGroup == "" {
		made, _, err := t.addMappings(ctx, opt.Volume, opt.Mappings, opt.LUN)
		return made, err
	}
	one, err := t.addMap(ctx, opt.Volume, opt.InitiatorGroup, opt.TargetGroup, opt.LUN)
	if err != nil {
		return nil, err
	}
	return append(results, one), nil
}

// addMap makes one mapping.
func (t *Array) addMap(ctx context.Context, volume, initiatorGroup, targetGroup string, lun int) (any, error) {
	if initiatorGroup == "" {
		return nil, fmt.Errorf("--initiatorgroup is mandatory")
	}
	data := map[string]any{
		"vol-id": volume,
		"ig-id":  initiatorGroup,
	}
	if targetGroup != "" {
		data["tg-id"] = targetGroup
	}
	if lun >= 0 {
		data["lun"] = lun
	}
	var answer struct {
		Content any `json:"content"`
	}
	if err := t.post(ctx, "/lun-maps", data, &answer); err != nil {
		return nil, err
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

// GetVolumeMappings returns the mappings of a volume.
func (t *Array) GetVolumeMappings(ctx context.Context, volume string) (lunMapsAnswer, error) {
	var answer lunMapsAnswer
	content, err := t.GetVolume(ctx, volume)
	if err != nil {
		return answer, err
	}
	params := map[string]string{"full": "1", "filter": "vol-name:eq:" + content.Name}
	err = t.get(ctx, "/lun-maps", params, &answer)
	return answer, err
}

// DelVolumeMappings unexports a volume from everything it is exported to.
func (t *Array) DelVolumeMappings(ctx context.Context, volume string) error {
	answer, err := t.GetVolumeMappings(ctx, volume)
	if err != nil {
		return err
	}
	for _, m := range answer.LunMaps {
		if err := t.DelMap(ctx, strconv.Itoa(m.Index)); err != nil {
			return err
		}
	}
	return nil
}
