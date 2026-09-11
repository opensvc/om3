package arrayhoc

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// The options this array needs and no other declares. Everything else comes
// from the catalogue in core/array, so a name and a size mean here what they
// mean on every other array.
//
// The names and the option sets are the ones v2 reads, so a command line
// written for the v2 agent runs here unchanged.
var (
	flagVolumeID = array.Flag{
		Name:    "id",
		Usage:   "item id",
		Kind:    array.Int,
		Default: -1,
	}
	flagVolumeIDRangeFrom = array.Flag{
		Name:    "start-ldev-id",
		Usage:   "volume id range begin",
		Kind:    array.Int,
		Default: -1,
	}
	flagVolumeIDRangeTo = array.Flag{
		Name:    "end-ldev-id",
		Usage:   "volume id range end",
		Kind:    array.Int,
		Default: -1,
	}
	flagResourceGroup = array.Flag{
		Name:  "resource-group",
		Usage: "resource group name",
		Kind:  array.String,
	}
	flagVirtualStorageMachineID = array.Flag{
		Name:  "vsm-id",
		Usage: "the optional virtual storage machine id",
		Kind:  array.String,
	}
	flagCompression = array.Flag{
		Name:  "compression",
		Usage: "activate volume compression",
		Kind:  array.Bool,
	}
	flagDeduplication = array.Flag{
		Name:  "dedup",
		Usage: "activate volume data deduplication",
		Kind:  array.Bool,
	}
	flagFilter = array.Flag{
		Name:  "filter",
		Usage: "filter the resources returned by a get query",
		Kind:  array.String,
	}
)

// optVolume reads the three ways an existing volume is named.
func optVolume(in array.Input) OptVolume {
	return OptVolume{
		ID:   in.Int(flagVolumeID.Name),
		Name: in.String(array.FlagName.Name),
	}
}

// optMapping reads how a volume is exported.
func optMapping(in array.Input) OptMapping {
	return OptMapping{
		Mappings:          in.StringSlice(array.FlagMapping.Name),
		LUN:               in.Int(array.FlagLUN.Name),
		HostGroupNames:    in.StringSlice(array.FlagTarget.Name),
		VolumeIdRangeFrom: in.Int(flagVolumeIDRangeFrom.Name),
		VolumeIdRangeTo:   in.Int(flagVolumeIDRangeTo.Name),
	}
}

// volumeFlags is the options naming an existing volume. v2 names one by id,
// naa or name, and the naa is not among them because this driver knows
// nothing of a naa yet.
var volumeFlags = []array.Flag{flagVolumeID, array.FlagName}

// mappingFlags is the options saying how a volume is exported, as v2 spells
// them for add_disk: a target to export through, or a mapping naming both
// ends of the path.
var mappingFlags = []array.Flag{
	array.FlagTarget,
	array.FlagMapping,
	array.FlagLUN,
	flagVolumeIDRangeFrom,
	flagVolumeIDRangeTo,
}

// Actions returns what this array answers to.
func (t *Array) Actions() []array.Action {
	getItems := func(path []string, short string, fn func(context.Context, OptGetItems) (any, error)) array.Action {
		return array.Action{
			Path:  path,
			Short: short,
			Flags: []array.Flag{flagFilter},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return fn(ctx, OptGetItems{Filter: in.String(flagFilter.Name)})
			},
		}
	}
	return []array.Action{
		{
			Path:  []string{"add", "disk"},
			Short: "add a volume and map",
			Flags: append([]array.Flag{
				array.FlagName,
				array.FlagSize,
				array.FlagPool,
				flagResourceGroup,
				flagCompression,
				flagDeduplication,
				flagVirtualStorageMachineID,
			}, mappingFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddDisk(ctx, OptAddDisk{
					Volume: OptAddVolume{
						Name:                    in.String(array.FlagName.Name),
						Size:                    in.String(array.FlagSize.Name),
						PoolId:                  in.String(array.FlagPool.Name),
						Compression:             in.Bool(flagCompression.Name),
						Deduplication:           in.Bool(flagDeduplication.Name),
						VirtualStorageMachineId: in.String(flagVirtualStorageMachineID.Name),
					},
					Mapping: optMapping(in),
				})
			},
		},
		{
			Path:  []string{"resize", "disk"},
			Short: "resize a volume",
			Flags: append([]array.Flag{array.FlagSize}, volumeFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.ResizeDisk(ctx, OptResizeDisk{
					Volume: optVolume(in),
					Size:   in.String(array.FlagSize.Name),
				})
			},
		},
		{
			Path:  []string{"del", "disk"},
			Short: "unmap a volume and delete",
			Flags: volumeFlags,
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, OptDelDisk{Volume: optVolume(in)})
			},
		},
		{
			Path:  []string{"map", "disk"},
			Short: "map a volume",
			Flags: append(append([]array.Flag{}, volumeFlags...), mappingFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.MapDisk(ctx, OptMapDisk{
					Volume:  optVolume(in),
					Mapping: optMapping(in),
				})
			},
		},
		{
			Path:  []string{"unmap", "disk"},
			Short: "unmap a volume",
			Flags: append(append([]array.Flag{}, volumeFlags...), array.FlagMapping),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.UnmapDisk(ctx, OptUnmapDisk{
					Volume: optVolume(in),
					Mapping: OptMapping{
						Mappings: in.StringSlice(array.FlagMapping.Name),
					},
				})
			},
		},
		{
			Path:  []string{"get", "volumes"},
			Short: "get volumes",
			Flags: append([]array.Flag{flagFilter}, volumeFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.GetVolumes(ctx, OptGetItems{
					Volume: optVolume(in),
					Filter: in.String(flagFilter.Name),
				})
			},
		},
		{
			Path:  []string{"get", "storage-system"},
			Short: "get storage system",
			Run: func(ctx context.Context, _ array.Input) (any, error) {
				return t.GetStorageSystem(ctx)
			},
		},
		getItems([]string{"get", "servers"}, "get servers",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetServers(ctx, opt) }),
		getItems([]string{"get", "host-groups"}, "get host groups",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetHostGroups(ctx, opt) }),
		getItems([]string{"get", "storage-ports"}, "get storage ports",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetStoragePorts(ctx, opt) }),
		getItems([]string{"get", "storage-pools"}, "get storage pools",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetStoragePools(ctx, opt) }),
		getItems([]string{"get", "disks"}, "get disks",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetDisks(ctx, opt) }),
		getItems([]string{"get", "storage-systems"}, "get storage systems",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetStorageSystems(ctx, opt) }),
		getItems([]string{"get", "jobs"}, "get jobs",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetJobs(ctx, opt) }),
		getItems([]string{"get", "system-tasks"}, "get system tasks",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetSystemTasks(ctx, opt) }),
	}
}

// Reports returns the sections of its configuration this array pushes to the
// collector.
//
// The section names are the ones v2 pushes for an hcs array, because the
// collector reads them to know what it was handed: a collector serving both
// agents stores what either of them pushes in the same place.
func (t *Array) Reports() []array.Report {
	return []array.Report{
		{
			Key: "system",
			Get: func(ctx context.Context) (any, error) {
				return t.GetStorageSystem(ctx)
			},
		},
		{
			Key: "pools",
			Get: func(ctx context.Context) (any, error) {
				return t.GetStoragePools(ctx, OptGetItems{})
			},
		},
		{
			Key: "fc_ports",
			Get: func(ctx context.Context) (any, error) {
				return t.GetStoragePorts(ctx, OptGetItems{})
			},
		},
		{
			Key: "ldevs",
			Get: func(ctx context.Context) (any, error) {
				return t.GetVolumes(ctx, OptGetItems{})
			},
		},
	}
}
