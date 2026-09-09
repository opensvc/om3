package arraypure

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// The options this array needs and no other declares. Everything else comes
// from the catalogue in core/array, so a name and a size mean here what they
// mean on every other array.
//
// The spellings are the ones this driver shipped, which are also v2's for
// everything but the plural of the mappings.
var (
	flagHost = array.Flag{
		Name:  "host",
		Usage: "initiator host name",
		Kind:  array.String,
	}
	flagNow = array.Flag{
		Name:  "now",
		Usage: "delete item after flagging it destroyed (DANGER)",
		Kind:  array.Bool,
	}
	flagPod = array.Flag{
		Name:  "pod",
		Usage: "pod name",
		Kind:  array.String,
	}
	flagFilter = array.Flag{
		Name:  "filter",
		Usage: "items filtering expression. ex: id='1' and serial='abc' and pod.name='pod1' and destroyed='false'.",
		Kind:  array.String,
	}
	flagSerial = array.Flag{
		Name:  "serial",
		Usage: "item serial",
		Kind:  array.String,
	}
	flagInitiator = array.Flag{
		Name:  "initiator",
		Usage: "initiator hba ids",
		Kind:  array.RawStringSlice,
	}
	flagTarget = array.Flag{
		Name:  "target",
		Usage: "targets to export the disk through",
		Kind:  array.RawStringSlice,
	}

	// This array names one host group, where another names several, so the
	// option is its own rather than the catalogue's.
	flagHostGroup = array.Flag{
		Name:  "hostgroup",
		Usage: "host group name",
		Kind:  array.String,
	}
)

// optVolume reads the three ways an existing volume is named.
func optVolume(in array.Input) OptVolume {
	return OptVolume{
		ID:     in.String(array.FlagID.Name),
		Name:   in.String(array.FlagName.Name),
		Serial: in.String(flagSerial.Name),
	}
}

// optMapping reads how a volume is exported.
func optMapping(in array.Input) OptMapping {
	return OptMapping{
		Mappings:      in.StringSlice(array.FlagMapping.Name),
		HostName:      in.String(flagHost.Name),
		HostGroupName: in.String(flagHostGroup.Name),
		LUN:           in.Int(array.FlagLUN.Name),
	}
}

// volumeFlags is the options naming an existing volume.
var volumeFlags = []array.Flag{array.FlagID, array.FlagName, flagSerial}

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
			Flags: []array.Flag{array.FlagName, array.FlagSize, array.FlagMapping, array.FlagLUN},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddDisk(ctx, OptAddDisk{
					Name:     in.String(array.FlagName.Name),
					Size:     in.String(array.FlagSize.Name),
					Mappings: in.StringSlice(array.FlagMapping.Name),
					LUN:      in.Int(array.FlagLUN.Name),
				})
			},
		},
		{
			Path:  []string{"resize", "disk"},
			Short: "resize a volume",
			Flags: append([]array.Flag{array.FlagSize, array.FlagTruncate}, volumeFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.ResizeDisk(ctx, OptResizeDisk{
					Volume:   optVolume(in),
					Size:     in.String(array.FlagSize.Name),
					Truncate: in.Bool(array.FlagTruncate.Name),
				})
			},
		},
		{
			Path:  []string{"del", "disk"},
			Short: "unmap a volume and delete",
			Flags: append([]array.Flag{flagNow}, volumeFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, OptDelDisk{
					Volume: optVolume(in),
					Now:    in.Bool(flagNow.Name),
				})
			},
		},
		{
			Path:  []string{"map", "disk"},
			Short: "map a volume",
			Flags: append(append([]array.Flag{}, volumeFlags...),
				array.FlagMapping, array.FlagLUN, flagHost, flagHostGroup),
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
			Flags: append(append([]array.Flag{}, volumeFlags...),
				array.FlagMapping, flagHost, flagHostGroup),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.UnmapDisk(ctx, OptUnmapDisk{
					Volume:  optVolume(in),
					Mapping: optMapping(in),
				})
			},
		},
		getItems([]string{"get", "hosts"}, "get hosts",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetHosts(ctx, opt) }),
		getItems([]string{"get", "connections"}, "get connections",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetConnections(ctx, opt) }),
		getItems([]string{"get", "volumes"}, "get volumes",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetVolumes(ctx, opt) }),
		getItems([]string{"get", "controllers"}, "get controllers",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetControllers(ctx, opt) }),
		getItems([]string{"get", "drives"}, "get drives",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetDrives(ctx, opt) }),
		getItems([]string{"get", "pods"}, "get pods",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetPods(ctx, opt) }),
		getItems([]string{"get", "ports"}, "get ports",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetPorts(ctx, opt) }),
		getItems([]string{"get", "interfaces"}, "get network interfaces",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetNetworkInterfaces(ctx, opt) }),
		getItems([]string{"get", "volumegroups"}, "get volume groups",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetVolumeGroups(ctx, opt) }),
		getItems([]string{"get", "hostgroups"}, "get host groups",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetHostGroups(ctx, opt) }),
		getItems([]string{"get", "arrays"}, "get arrays",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetArrays(ctx, opt) }),
		getItems([]string{"get", "hardware"}, "get hardware",
			func(ctx context.Context, opt OptGetItems) (any, error) { return t.GetHardware(ctx, opt) }),
	}
}
