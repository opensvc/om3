package arrayfreenas

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/array"
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
//
// The blocksize is not one of them: it is the one the extent exports, and
// the zvol is left the volblocksize the array chooses, as v2 leaves it.
var zvolFlags = []array.Flag{
	array.FlagName,
	array.FlagSize,
	flagSparse,
	flagDedup,
	flagCompression,
}

// diskFlags is the options describing a disk to create: a zvol and how it is
// exported.
var diskFlags = append(append([]array.Flag{}, zvolFlags...),
	flagBlocksizeInt, flagInsecureTPC, array.FlagLUN, array.FlagMapping)

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
			Flags: diskFlags,
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.addDiskReport(ctx, optAddDisk(in, in.String(array.FlagName.Name)))
			},
		},
		{
			// The name of the dataset is the volume and the name together,
			// and the extent is named by the name alone, as v2 names it: the
			// collector knows the disk by this name, and deletes and resizes
			// it by it. The collector writes this one, so it stays answered,
			// and out of the help so nobody else starts writing it.
			Path:   []string{"add", "iscsi", "zvol"},
			Short:  "add a zvol-type dataset",
			Hidden: true,
			Flags:  append([]array.Flag{flagVolume}, diskFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				volume, name := in.String(flagVolume.Name), in.String(array.FlagName.Name)
				if volume == "" || name == "" {
					return nil, fmt.Errorf("--volume and --name are required")
				}
				opt := optAddDisk(in, volume+"/"+name)
				opt.ExtentName = name
				return t.addDiskReport(ctx, opt)
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
				return t.DelZvol(ctx, in.String(array.FlagName.Name))
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
			Flags: []array.Flag{array.FlagName, array.FlagSize, array.FlagTruncate},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.resizeZvol(ctx, in.String(array.FlagName.Name), in.String(array.FlagSize.Name), in.Bool(array.FlagTruncate.Name))
			},
		},
		{
			Path:  []string{"update", "dataset"},
			Short: "update a dataset",
			Flags: []array.Flag{array.FlagName, array.FlagSize, array.FlagTruncate},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.resizeZvol(ctx, in.String(array.FlagName.Name), in.String(array.FlagSize.Name), in.Bool(array.FlagTruncate.Name))
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

// resizeZvol resizes the zvol a command line names, read as resolveZvol
// reads it: the collector names a disk by its extent name. A size beginning
// with "+" is added to the size the zvol has. A size below the current one
// is refused unless truncate is set: the array drops the end of the zvol.
func (t *Array) resizeZvol(ctx context.Context, name, size string, truncate bool) (*Dataset, error) {
	newSize, err := array.ParseSize(size)
	if err != nil {
		return nil, err
	}
	ref, err := t.resolveZvol(ctx, name)
	if err != nil {
		return nil, err
	}
	if ref.dataset.Volsize == nil {
		return nil, fmt.Errorf("zvol %s: the array reports no volsize", ref.dataset.Name)
	}
	current, err := strconv.ParseInt(ref.dataset.Volsize.Rawvalue, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("zvol %s: volsize %q: %w", ref.dataset.Name, ref.dataset.Volsize.Rawvalue, err)
	}
	target := newSize.Target(current)
	if err := array.CheckResize(current, target, truncate); err != nil {
		return nil, fmt.Errorf("zvol %s: %w", ref.dataset.Name, err)
	}
	return t.UpdateDataset(ctx, ref.dataset.Id, UpdateDatasetParams{Volsize: &target})
}

// AddDiskReport is what an add reports, in the keys v2 reports it with: the
// collector stores it as the result of its disk form. The dataset and the
// targetextents are reported too, for a reader of the command line.
type AddDiskReport struct {
	DriverData    ISCSIExtent        `json:"driver_data"`
	DiskID        string             `json:"disk_id"`
	DiskDevID     int                `json:"disk_devid"`
	Mappings      []DiskMapping      `json:"mappings"`
	Warnings      []string           `json:"warnings,omitempty"`
	Dataset       *Dataset           `json:"dataset,omitempty"`
	TargetExtents ISCSITargetExtents `json:"targetextents"`
}

// DiskMapping is an initiator reaching a disk through a target, as v2's
// list_mappings reports it.
type DiskMapping struct {
	TargetGroup ISCSITargetGroup  `json:"targetgroup"`
	Extent      ISCSITargetExtent `json:"extent"`
	DiskID      string            `json:"disk_id"`
	TgtID       string            `json:"tgt_id"`
	HBAID       string            `json:"hba_id"`
}

// addDiskReport adds a disk and reports it as v2 does.
//
// The mappings are read back from the array once the disk is made. Failing
// to read them is a warning, not an error: the disk exists and is exported,
// and a caller told the add failed forgets a disk the array serves.
func (t *Array) addDiskReport(ctx context.Context, opt AddDiskOptions) (*AddDiskReport, error) {
	disk, err := t.AddDisk(ctx, opt)
	if err != nil {
		return nil, err
	}
	extent := *disk.ISCSI.Extent
	report := AddDiskReport{
		DriverData:    extent,
		DiskID:        t.DiskId(*disk),
		DiskDevID:     extent.Id,
		Mappings:      make([]DiskMapping, 0),
		Dataset:       disk.Dataset,
		TargetExtents: disk.ISCSI.TargetExtents,
	}
	if mappings, err := t.diskMappings(ctx, extent); err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("list the mappings of extent %d: %s", extent.Id, err))
	} else {
		report.Mappings = mappings
	}
	return &report, nil
}

// diskMappings returns the initiators reaching an extent, and the targets
// they reach it through, sorted as v2 sorts them. They are the initiators
// the target groups of its targets allow, not the ones a command line asked
// for: they are who sees the disk.
func (t *Array) diskMappings(ctx context.Context, extent ISCSIExtent) ([]DiskMapping, error) {
	targetExtents, err := t.GetISCSITargetExtents(ctx)
	if err != nil {
		return nil, err
	}
	targets, err := t.GetISCSITargets(ctx)
	if err != nil {
		return nil, err
	}
	initiators, err := t.GetISCSIInitiators(ctx)
	if err != nil {
		return nil, err
	}
	diskID := strings.TrimPrefix(extent.NAA, "0x")
	byKey := make(map[string]DiskMapping)
	for _, targetExtent := range targetExtents.WithExtent(extent) {
		target, ok := targets.GetById(targetExtent.TargetId)
		if !ok {
			return nil, fmt.Errorf("target id %d of targetextent %d not found", targetExtent.TargetId, targetExtent.Id)
		}
		for _, group := range target.Groups {
			names, any, err := groupInitiators(group, initiators)
			if err != nil {
				return nil, fmt.Errorf("target %s: %w", target.Name, err)
			}
			if any {
				// Every initiator sees the disk through this target, which
				// the report says rather than listing none.
				names = []string{anyInitiator}
			}
			for _, hba := range names {
				byKey[hba+":"+target.Name+":"+diskID] = DiskMapping{
					TargetGroup: group,
					Extent:      targetExtent,
					DiskID:      diskID,
					TgtID:       target.Name,
					HBAID:       hba,
				}
			}
		}
	}
	l := make([]DiskMapping, 0, len(byKey))
	for _, m := range byKey {
		l = append(l, m)
	}
	sort.Slice(l, func(i, j int) bool {
		if l[i].HBAID != l[j].HBAID {
			return l[i].HBAID < l[j].HBAID
		}
		return l[i].TgtID < l[j].TgtID
	})
	return l, nil
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
