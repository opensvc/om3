package arrayxtremio

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// The options this array needs and no other declares. Everything else comes
// from the catalogue in core/array.
//
// The names and the option sets are the ones v2 reads, so a command line
// written for the v2 agent, and the one the collector writes, run here
// unchanged.
var (
	flagVolume = array.Flag{
		Name:  "volume",
		Usage: "a volume name or id",
		Kind:  array.String,
	}
	flagInitiatorGroup = array.Flag{
		Name:  "initiatorgroup",
		Usage: "the initiator group id or name",
		Kind:  array.String,
	}
	flagTargetGroup = array.Flag{
		Name:  "targetgroup",
		Usage: "a target group name or id",
		Kind:  array.String,
	}
	flagMapping = array.Flag{
		Name:  "mapping",
		Usage: "a lun mapping index",
		Kind:  array.String,
	}
	flagInitiator = array.Flag{
		Name:  "initiator",
		Usage: "an initiator id or name",
		Kind:  array.String,
	}
	flagTarget = array.Flag{
		Name:  "target",
		Usage: "a target id or name",
		Kind:  array.String,
	}
	flagTag = array.Flag{
		Name:  "tag",
		Usage: "an object tag, can be set multiple times",
		Kind:  array.RawStringSlice,
	}
	flagBlocksize = array.Flag{
		Name:  "blocksize",
		Usage: "the exported disk blocksize in B",
		Kind:  array.Int,
	}
	flagAlignmentOffset = array.Flag{
		Name:  "alignment-offset",
		Usage: "indicates the offset of the disk partition on the volume",
		Kind:  array.Int,
	}
	flagSmallIOAlerts = array.Flag{
		Name:  "small-io-alerts",
		Usage: "enable or disable the small input/output alerts",
		Kind:  array.String,
	}
	flagUnalignedIOAlerts = array.Flag{
		Name:  "unaligned-io-alerts",
		Usage: "enable or disable the unaligned input/output alerts",
		Kind:  array.String,
	}
	flagVAAITPAlerts = array.Flag{
		Name:  "vaai-tp-alerts",
		Usage: "enable or disable the vaai thin provisioning alerts",
		Kind:  array.String,
	}
	flagAccess = array.Flag{
		Name:  "access",
		Usage: "the volume access rights: no_access, read_access or write_access",
		Kind:  array.String,
	}
)

// Actions returns what this array answers to.
func (t *Array) Actions() []array.Action {
	list := func(path []string, short string, resource string, flag array.Flag, filter string) array.Action {
		return array.Action{
			Path:  path,
			Short: short,
			Flags: []array.Flag{flag},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				params := map[string]string{"full": "1"}
				if v := in.String(flag.Name); v != "" {
					params["filter"] = filter + ":eq:" + v
				}
				var data any
				err := t.get(ctx, resource, params, &data)
				return data, err
			},
		}
	}
	return []array.Action{
		{
			Path:  []string{"add", "disk"},
			Short: "add a volume and export it",
			Flags: []array.Flag{
				array.FlagName,
				array.FlagSize,
				flagBlocksize,
				flagTag,
				flagAlignmentOffset,
				flagSmallIOAlerts,
				flagUnalignedIOAlerts,
				flagVAAITPAlerts,
				flagAccess,
				array.FlagMapping,
			},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddDisk(ctx, OptAddDisk{
					Name:              in.String(array.FlagName.Name),
					Size:              in.String(array.FlagSize.Name),
					Blocksize:         in.Int(flagBlocksize.Name),
					Tags:              in.StringSlice(flagTag.Name),
					AlignmentOffset:   in.Int(flagAlignmentOffset.Name),
					SmallIOAlerts:     in.String(flagSmallIOAlerts.Name),
					UnalignedIOAlerts: in.String(flagUnalignedIOAlerts.Name),
					VAAITPAlerts:      in.String(flagVAAITPAlerts.Name),
					Access:            in.String(flagAccess.Name),
					Mappings:          in.StringSlice(array.FlagMapping.Name),
				})
			},
		},
		{
			Path:  []string{"del", "disk"},
			Short: "unexport a volume and delete it",
			Flags: []array.Flag{flagVolume},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, in.String(flagVolume.Name))
			},
		},
		{
			Path:  []string{"resize", "disk"},
			Short: "resize a volume",
			Flags: []array.Flag{flagVolume, array.FlagSize},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.ResizeDisk(ctx, in.String(flagVolume.Name), in.String(array.FlagSize.Name))
			},
		},
		{
			Path:  []string{"add", "map"},
			Short: "export a volume",
			Flags: []array.Flag{flagVolume, array.FlagMapping, flagInitiatorGroup, flagTargetGroup, array.FlagLUN},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddMap(ctx, OptAddMap{
					Volume:         in.String(flagVolume.Name),
					Mappings:       in.StringSlice(array.FlagMapping.Name),
					InitiatorGroup: in.String(flagInitiatorGroup.Name),
					TargetGroup:    in.String(flagTargetGroup.Name),
					LUN:            in.Int(array.FlagLUN.Name),
				})
			},
		},
		{
			Path:  []string{"del", "map"},
			Short: "unexport a volume",
			Flags: []array.Flag{flagMapping, flagVolume, flagInitiatorGroup, flagTargetGroup},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				if mapping := in.String(flagMapping.Name); mapping != "" {
					return nil, t.DelMap(ctx, mapping)
				}
				// v2 takes the mapping index and nothing else here. Naming
				// the volume instead removes every mapping of it, which is
				// what deleting a disk does on the way.
				if volume := in.String(flagVolume.Name); volume != "" {
					return nil, t.DelVolumeMappings(ctx, volume)
				}
				return nil, errMappingRequired
			},
		},
		list([]string{"list", "volumes"}, "list volumes", "/volumes", flagVolume, "name"),
		list([]string{"list", "initiators"}, "list initiators", "/initiators", flagInitiator, "name"),
		list([]string{"list", "initiator-groups"}, "list initiator groups", "/initiator-groups", flagInitiatorGroup, "name"),
		list([]string{"list", "targets"}, "list targets", "/targets", flagTarget, "name"),
		list([]string{"list", "target-groups"}, "list target groups", "/target-groups", flagTargetGroup, "name"),
		list([]string{"list", "mappings"}, "list mappings", "/lun-maps", flagVolume, "vol-name"),
	}
}

// Reports returns the sections of its configuration this array pushes to the
// collector.
//
// The section names are the ones v2 pushes for an xtremio array, because the
// collector reads them to know what it was handed.
func (t *Array) Reports() []array.Report {
	details := func(resource, key string) func(context.Context) (any, error) {
		return func(ctx context.Context) (any, error) {
			var data map[string]any
			if err := t.get(ctx, resource, map[string]string{"full": "1"}, &data); err != nil {
				return nil, err
			}
			return data[key], nil
		}
	}
	return []array.Report{
		{Key: "clusters_details", Get: details("/clusters", "clusters")},
		{Key: "volumes_details", Get: details("/volumes", "volumes")},
		{Key: "targets_details", Get: details("/targets", "targets")},
	}
}
