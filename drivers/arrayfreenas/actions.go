package arrayfreenas

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

// The options this array needs and no other declares. Everything else comes
// from the catalogue in core/array, so a name and a size mean here what they
// mean on every other array.
var (
	flagVolume = array.Flag{
		Name:  "volume",
		Usage: "the volume to create the disk into",
		Kind:  array.String,
	}
	flagSparse = array.Flag{
		Name:  "sparse",
		Usage: "create a sparse zvol",
		Kind:  array.Bool,
	}
	flagDedup = array.Flag{
		Name:    "dedup",
		Usage:   "toggle deduplication: on, off, verify",
		Kind:    array.String,
		Default: "off",
	}
	flagCompression = array.Flag{
		Name:    "compression",
		Usage:   "toggle compression: off, inherit, lz4, gzip, gzip-9, zle",
		Kind:    array.String,
		Default: "inherit",
	}
	flagInsecureTPC = array.Flag{
		Name:    "insecure-tpc",
		Usage:   "allow initiators to xcopy without authenticating to foreign targets",
		Kind:    array.Bool,
		Default: true,
	}
	flagDisk = array.Flag{
		Name:  "disk",
		Usage: "the disk to serve the extent from",
		Kind:  array.String,
	}
	flagComment = array.Flag{
		Name:  "comment",
		Usage: "a description for your reference",
		Kind:  array.String,
	}
	flagAuthGroupID = array.Flag{
		Name:    "auth-group-id",
		Usage:   "the auth group object id",
		Kind:    array.Int,
		Default: -1,
	}
	flagAuthMethod = array.Flag{
		Name:    "auth-method",
		Usage:   "NONE, CHAP or CHAP Mutual",
		Kind:    array.String,
		Default: "NONE",
	}
	flagAuth = array.Flag{
		Name:  "auth",
		Usage: "the auth group id",
		Kind:  array.String,
	}
	flagListen = array.Flag{
		Name:  "listen",
		Usage: "an <ip>:<port> the portal listens on, can be set multiple times",
		Kind:  array.RawStringSlice,
	}
	flagPortalID = array.Flag{
		Name:    "portal-id",
		Usage:   "the portal object id",
		Kind:    array.Int,
		Default: -1,
	}
	flagTarget = array.Flag{
		Name:  "target",
		Usage: "the target object name",
		Kind:  array.String,
	}
	flagInitiatorName = array.Flag{
		Name:  "initiatorgroup",
		Usage: "the initiator group name",
		Kind:  array.String,
	}
	flagInitiatorID = array.Flag{
		Name:    "initiator-id",
		Usage:   "the initiator group object id",
		Kind:    array.Int,
		Default: -1,
	}
	flagInitiators = array.Flag{
		Name:  "initiator",
		Usage: "an initiator iqn, can be set multiple times",
		Kind:  array.RawStringSlice,
	}
	flagAuthNetworks = array.Flag{
		Name:  "auth-network",
		Usage: "a network authorized to access the target, ip or cidr, or ALL",
		Kind:  array.RawStringSlice,
	}
	flagBlocksizeInt = array.Flag{
		Name:    "blocksize",
		Usage:   "the exported disk blocksize in B",
		Kind:    array.String,
		Default: "512",
	}
	flagID = array.Flag{
		Name:    "id",
		Usage:   "an object id, as reported by a get action",
		Kind:    array.Int,
		Default: -1,
	}
)

// zvolFlags is the options describing a zvol to create.
var zvolFlags = []array.Flag{
	array.FlagName,
	flagBlocksizeInt,
	array.FlagSize,
	flagSparse,
	flagDedup,
	flagCompression,
}

// optAddZvol reads what a zvol is made of.
func optAddZvol(in array.Input, name string) AddZvolOptions {
	return AddZvolOptions{
		Name:          name,
		Size:          in.String(array.FlagSize.Name),
		Blocksize:     in.String(flagBlocksizeInt.Name),
		Sparse:        in.Bool(flagSparse.Name),
		Deduplication: in.String(flagDedup.Name),
		Compression:   in.String(flagCompression.Name),
	}
}

// optAddDisk reads what a disk is made of, which is a zvol and how it is
// exported.
func optAddDisk(in array.Input, name string) AddDiskOptions {
	opt := AddDiskOptions{
		AddZvolOptions: optAddZvol(in, name),
		InsecureTPC:    in.Bool(flagInsecureTPC.Name),
		Mappings:       in.StringSlice(array.FlagMapping.Name),
	}
	if lun := in.Int(array.FlagLUN.Name); lun >= 0 {
		opt.LunId = &lun
	}
	return opt
}

// Actions returns what this array answers to.
func (t *Array) Actions() []array.Action {
	dumper := func(path []string, short string, fn func(context.Context) error) array.Action {
		return array.Action{
			Path:  path,
			Short: short,
			Run: func(ctx context.Context, _ array.Input) (any, error) {
				return nil, fn(ctx)
			},
		}
	}
	dumperNamed := func(path []string, short string, fn func(context.Context, string) error) array.Action {
		return array.Action{
			Path:  path,
			Short: short,
			Flags: []array.Flag{array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return nil, fn(ctx, in.String(array.FlagName.Name))
			},
		}
	}
	mapDisk := func(path []string, hidden bool) array.Action {
		return array.Action{
			Path:   path,
			Short:  "map a zvol-type dataset",
			Hidden: hidden,
			Flags:  []array.Flag{array.FlagName, array.FlagLUN, array.FlagMapping},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				opt := MapDiskOptions{
					Name:     in.String(array.FlagName.Name),
					Mappings: in.StringSlice(array.FlagMapping.Name),
				}
				if lun := in.Int(array.FlagLUN.Name); lun >= 0 {
					opt.LunId = &lun
				}
				return t.MapDisk(ctx, opt)
			},
		}
	}

	return []array.Action{
		{
			Path:  []string{"add", "disk"},
			Short: "add a zvol-type dataset and map",
			Flags: append(append([]array.Flag{}, zvolFlags...),
				flagInsecureTPC, array.FlagLUN, array.FlagMapping),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddDisk(ctx, optAddDisk(in, in.String(array.FlagName.Name)))
			},
		},
		{
			// The name of the dataset is the volume and the name together.
			// The collector writes this one, so it stays answered, and out of
			// the help so nobody else starts writing it.
			Path:   []string{"add", "iscsi", "zvol"},
			Short:  "add a zvol-type dataset",
			Hidden: true,
			Flags: append(append([]array.Flag{flagVolume}, zvolFlags...),
				flagInsecureTPC, array.FlagLUN, array.FlagMapping),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				name := in.String(flagVolume.Name) + "/" + in.String(array.FlagName.Name)
				return t.AddDisk(ctx, optAddDisk(in, name))
			},
		},
		{
			Path:  []string{"add", "zvol"},
			Short: "add a zvol-type dataset",
			Flags: zvolFlags,
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddZvol(ctx, optAddZvol(in, in.String(array.FlagName.Name)))
			},
		},
		{
			Path:  []string{"del", "disk"},
			Short: "unmap a zvol-type dataset and delete",
			Flags: []array.Flag{array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, in.String(array.FlagName.Name))
			},
		},
		{
			Path:  []string{"del", "zvol"},
			Short: "del a zvol-type dataset",
			Flags: []array.Flag{array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DeleteDataset(ctx, in.String(array.FlagName.Name))
			},
		},
		{
			Path:  []string{"del", "iscsi", "zvol"},
			Short: "unmap a zvol-type dataset and delete",
			// The collector writes this one too.
			Hidden: true,
			Flags:  []array.Flag{array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, in.String(array.FlagName.Name))
			},
		},
		mapDisk([]string{"map", "disk"}, false),
		mapDisk([]string{"map", "iscsi", "zvol"}, true),
		{
			Path:  []string{"unmap", "iscsi", "zvol"},
			Short: "unmap a zvol-type dataset",
			Flags: []array.Flag{array.FlagName, array.FlagMapping},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.UnmapDisk(ctx, UnmapDiskOptions{
					Name:     in.String(array.FlagName.Name),
					Mappings: in.StringSlice(array.FlagMapping.Name),
				})
			},
		},
		{
			// The collector writes this one, and v2 answers to it.
			Path:  []string{"resize", "zvol"},
			Short: "resize a zvol-type dataset",
			Flags: []array.Flag{array.FlagName, array.FlagSize},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.updateDatasetSize(ctx, in.String(array.FlagName.Name), in.String(array.FlagSize.Name))
			},
		},
		{
			Path:  []string{"update", "dataset"},
			Short: "update a dataset",
			Flags: []array.Flag{array.FlagName, array.FlagSize},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.updateDatasetSize(ctx, in.String(array.FlagName.Name), in.String(array.FlagSize.Name))
			},
		},
		{
			Path:  []string{"add", "iscsi", "portal"},
			Short: "create a iscsi portal",
			Flags: []array.Flag{flagComment, flagAuthGroupID, flagAuthMethod, flagListen},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				listen, err := parseListen(in.StringSlice(flagListen.Name))
				if err != nil {
					return nil, err
				}
				params := CreateISCSIPortalParams{
					Comment:             in.String(flagComment.Name),
					DiscoveryAuthMethod: in.String(flagAuthMethod.Name),
					Listen:              listen,
				}
				if id := in.Int(flagAuthGroupID.Name); id >= 0 {
					params.DiscoveryAuthGroup = id
				}
				return t.addISCSIPortal(ctx, params)
			},
		},
		{
			Path:  []string{"add", "iscsi", "target"},
			Short: "create a iscsi target",
			Flags: []array.Flag{array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.addISCSITarget(ctx, CreateISCSITargetParams{Name: in.String(array.FlagName.Name)})
			},
		},
		{
			Path:  []string{"add", "iscsi", "targetgroup"},
			Short: "create a iscsi targetgroup",
			Flags: []array.Flag{flagPortalID, flagTarget, flagAuth, flagAuthMethod, flagInitiatorName, flagInitiators, flagInitiatorID},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.addISCSITargetGroup(ctx, AddISCSITargetGroupOptions{
					AuthMethod:    in.String(flagAuthMethod.Name),
					Auth:          in.String(flagAuth.Name),
					PortalId:      in.Int(flagPortalID.Name),
					Target:        in.String(flagTarget.Name),
					InitiatorName: in.String(flagInitiatorName.Name),
					InitiatorId:   in.Int(flagInitiatorID.Name),
				})
			},
		},
		{
			Path:  []string{"add", "iscsi", "initiator"},
			Short: "create a iscsi initiator",
			Flags: []array.Flag{flagComment, flagInitiators, flagAuthNetworks},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.addISCSIInitiator(ctx, CreateISCSIInitiatorParams{
					Initiators:  in.StringSlice(flagInitiators.Name),
					AuthNetwork: in.StringSlice(flagAuthNetworks.Name),
					Comment:     in.String(flagComment.Name),
				})
			},
		},
		{
			Path:  []string{"add", "iscsi", "extent"},
			Short: "create a iscsi extent",
			Flags: []array.Flag{array.FlagName, flagDisk, flagBlocksizeInt, flagInsecureTPC},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddISCSIExtent(ctx, AddISCSIExtentOptions{
					Name:        in.String(array.FlagName.Name),
					Disk:        in.String(flagDisk.Name),
					Blocksize:   in.String(flagBlocksizeInt.Name),
					InsecureTPC: in.Bool(flagInsecureTPC.Name),
				})
			},
		},
		{
			Path:  []string{"del", "iscsi", "extent"},
			Short: "delete a iscsi extent",
			Flags: []array.Flag{flagID, array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelISCSIExtent(ctx, DelISCSIExtentOptions{
					Id:   in.Int(flagID.Name),
					Name: in.String(array.FlagName.Name),
				})
			},
		},
		{
			Path:  []string{"del", "iscsi", "target"},
			Short: "delete a iscsi target",
			Flags: []array.Flag{flagID},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.delISCSITarget(ctx, in.Int(flagID.Name))
			},
		},
		{
			Path:  []string{"del", "iscsi", "initiator"},
			Short: "delete a iscsi initiator",
			Flags: []array.Flag{flagID},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.delISCSIInitiator(ctx, in.Int(flagID.Name))
			},
		},
		dumper([]string{"get", "pools"}, "get pools", t.dumpPools),
		dumper([]string{"get", "datasets"}, "get datasets", t.dumpDatasets),
		dumper([]string{"get", "system"}, "get system information", t.dumpSystemInfo),
		dumperNamed([]string{"get", "disk"}, "get dataset, extent and targetextents", t.dumpDisk),
		dumperNamed([]string{"get", "dataset"}, "get dataset", t.dumpDataset),
		dumper([]string{"get", "iscsi", "portals"}, "get iscsi portals", t.dumpISCSIPortals),
		dumper([]string{"get", "iscsi", "targets"}, "get iscsi targets", t.dumpISCSITargets),
		dumper([]string{"get", "iscsi", "targetextents"}, "get iscsi targetextents", t.dumpISCSITargetExtents),
		dumper([]string{"get", "iscsi", "extents"}, "get iscsi extents", t.dumpISCSIExtents),
		dumper([]string{"get", "iscsi", "initiators"}, "get iscsi initiators", t.dumpISCSIInitiators),
		dumperNamed([]string{"get", "iscsi", "extent"}, "get iscsi extent", t.dumpISCSIExtent),
	}
}

// parseListen reads the addresses a portal listens on.
func parseListen(l []string) ([]ISCSIPortalListenIp, error) {
	out := make([]ISCSIPortalListenIp, 0, len(l))
	for _, server := range l {
		ip, portString, ok := strings.Cut(server, ":")
		if !ok {
			return nil, fmt.Errorf("bad listen format: %s", server)
		}
		port, err := strconv.Atoi(portString)
		if err != nil {
			return nil, fmt.Errorf("bad listen port format: %s", server)
		}
		out = append(out, ISCSIPortalListenIp{Ip: ip, Port: port})
	}
	return out, nil
}

// updateDatasetSize resizes a dataset. A size beginning with a sign is added
// to, or taken from, the size the dataset has.
func (t *Array) updateDatasetSize(ctx context.Context, name, size string) (any, error) {
	var (
		params UpdateDatasetParams
		result int64
		sign   string
	)
	if strings.HasPrefix(size, "+") || strings.HasPrefix(size, "-") {
		sign = string(size[0])
		size = size[1:]
		ds, err := t.GetDataset(ctx, name)
		if err != nil {
			return nil, err
		}
		i, err := sizeconv.FromSize(ds.Volsize.Rawvalue)
		if err != nil {
			return nil, err
		}
		result = i
	}
	i, err := sizeconv.FromSize(size)
	if err != nil {
		return nil, err
	}
	switch sign {
	case "+":
		result += i
	case "-":
		result -= i
	default:
		result = i
	}
	params.Volsize = &result
	return t.UpdateDataset(ctx, name, params)
}

// Reports returns the sections of its configuration this array pushes to the
// collector.
//
// The section names are the ones v2 pushes for a freenas array, because the
// collector reads them to know what it was handed.
func (t *Array) Reports() []array.Report {
	return []array.Report{
		{
			Key: "version",
			Get: func(ctx context.Context) (any, error) {
				return t.GetSystemInfo(ctx)
			},
		},
		{
			Key: "volumes",
			Get: func(ctx context.Context) (any, error) {
				return t.GetDatasets(ctx)
			},
		},
		{
			Key: "iscsi_targets",
			Get: func(ctx context.Context) (any, error) {
				return t.GetISCSITargets(ctx)
			},
		},
		{
			Key: "iscsi_targettoextents",
			Get: func(ctx context.Context) (any, error) {
				return t.GetISCSITargetExtents(ctx)
			},
		},
		{
			Key: "iscsi_extents",
			Get: func(ctx context.Context) (any, error) {
				return t.GetISCSIExtents(ctx)
			},
		},
	}
}
