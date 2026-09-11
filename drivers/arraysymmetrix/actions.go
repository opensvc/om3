package arraysymmetrix

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// The options this array needs and no other declares. Everything else comes
// from the catalogue in core/array, so a name and a size mean here what they
// mean on every other array.
//
// The spellings are v2's.
var (
	flagDev = array.Flag{
		Name:  "dev",
		Usage: "the device id (ex: 00A04)",
		Kind:  array.String,
	}
	flagData = array.Flag{
		Name:  "data",
		Usage: "the workplan provided in json format",
		Kind:  array.String,
	}
	flagPair = array.Flag{
		Name:  "pair",
		Usage: "the device id pair (ex: 00A04:00A04)",
		Kind:  array.String,
	}
	flagSLO = array.Flag{
		Name:  "slo",
		Usage: "the thin device Service Level Objective",
		Kind:  array.String,
	}
	flagSRP = array.Flag{
		Name:  "srp",
		Usage: "the Storage Resource Pool hosting the device",
		Kind:  array.String,
	}
	flagSRDF = array.Flag{
		Name:  "srdf",
		Usage: "create a SRDF mirrored device pair with --array hosting the R1 member",
		Kind:  array.Bool,
	}
	flagSRDFType = array.Flag{
		Name:    "srdf-type",
		Usage:   "the device role in the SRDF mirror (ex: R1)",
		Kind:    array.String,
		Default: "R1",
	}
	flagSRDFMode = array.Flag{
		Name:    "srdf-mode",
		Usage:   "device mirroring mode. either sync, acp_wp or acp_disk",
		Kind:    array.String,
		Default: "sync",
	}
	flagRDFG = array.Flag{
		Name:  "rdfg",
		Usage: "the RDF / RA Group number, required if --srdf is set",
		Kind:  array.String,
	}
	flagSG = array.Flag{
		Name:  "sg",
		Usage: "as an alternative to --mappings, specify the storage group to put the dev into.",
		Kind:  array.String,
	}
	flagInvalidate = array.Flag{
		Name:  "invalidate",
		Usage: "the SRDF mirror member to invalidate upon createpair (ex: R2). don't set to just establish",
		Kind:  array.String,
	}
)

// srdfFlags is the options describing an SRDF mirror.
var srdfFlags = []array.Flag{flagSRDF, flagSRDFMode, flagSRDFType, flagRDFG}

// optMappings reads the mappings, in the grammar the collector writes them.
func optMappings(in array.Input) (array.Mappings, error) {
	return array.ParseMappings(in.StringSlice(array.FlagMapping.Name))
}

// Actions returns what this array answers to.
func (t *Array) Actions() []array.Action {
	dumper := func(path []string, short string, fn func(context.Context) (any, error)) array.Action {
		return array.Action{
			Path:  path,
			Short: short,
			Run: func(ctx context.Context, _ array.Input) (any, error) {
				return fn(ctx)
			},
		}
	}
	return []array.Action{
		{
			Path:  []string{"add", "disk"},
			Short: "add a volume and map",
			Flags: append([]array.Flag{
				array.FlagName, array.FlagSize, array.FlagMapping, flagSLO, flagSG, flagSRP,
			}, srdfFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				mappings, err := optMappings(in)
				if err != nil {
					return nil, err
				}
				return t.AddDisk(ctx, OptAddDisk{
					Name:     in.String(array.FlagName.Name),
					Size:     in.String(array.FlagSize.Name),
					SLO:      in.String(flagSLO.Name),
					SRP:      in.String(flagSRP.Name),
					SRDF:     in.Bool(flagSRDF.Name),
					SRDFMode: in.String(flagSRDFMode.Name),
					SRDFType: in.String(flagSRDFType.Name),
					RDFG:     in.String(flagRDFG.Name),
					Mappings: mappings,
				})
			},
		},
		{
			Path:  []string{"add", "tdev"},
			Short: "add a thin dev, no masking",
			Flags: append([]array.Flag{array.FlagName, array.FlagSize, flagSG}, srdfFlags...),
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddThinDev(ctx, OptAddThinDev{
					Name:     in.String(array.FlagName.Name),
					Size:     in.String(array.FlagSize.Name),
					SRDF:     in.Bool(flagSRDF.Name),
					RDFG:     in.String(flagRDFG.Name),
					SG:       in.String(flagSG.Name),
					SRDFMode: in.String(flagSRDFMode.Name),
					SRDFType: in.String(flagSRDFType.Name),
				})
			},
		},
		{
			Path:  []string{"del", "disk"},
			Short: "unmap a volume and delete",
			Flags: []array.Flag{flagDev},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, OptDelDisk{Dev: in.String(flagDev.Name)})
			},
		},
		{
			Path:  []string{"del", "tdev"},
			Short: "delete a thin dev, no unmasking",
			Flags: []array.Flag{flagDev},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelThinDev(ctx, OptDelThinDev{Dev: in.String(flagDev.Name)})
			},
		},
		{
			Path:  []string{"resize", "disk"},
			Short: "resize a volume",
			Flags: []array.Flag{flagDev, array.FlagSize, array.FlagForce},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.ResizeDisk(ctx, OptResizeDisk{
					Dev:   in.String(flagDev.Name),
					Size:  in.String(array.FlagSize.Name),
					Force: in.Bool(array.FlagForce.Name),
				})
			},
		},
		{
			Path:  []string{"rename", "disk"},
			Short: "rename a device",
			Flags: []array.Flag{flagDev, array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.RenameDisk(ctx, OptRenameDisk{
					Dev:  in.String(flagDev.Name),
					Name: in.String(array.FlagName.Name),
				})
			},
		},
		{
			Path:  []string{"map", "disk"},
			Short: "map a device",
			Flags: []array.Flag{flagDev, array.FlagMapping, flagSLO, flagSRP, flagSG},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				mappings, err := optMappings(in)
				if err != nil {
					return nil, err
				}
				return t.MapDisk(ctx, OptMapDisk{
					Dev:      in.String(flagDev.Name),
					Mappings: mappings,
					SLO:      in.String(flagSLO.Name),
					SRP:      in.String(flagSRP.Name),
					SG:       in.String(flagSG.Name),
				})
			},
		},
		{
			Path:  []string{"unmap", "disk"},
			Short: "unmap a volume",
			Flags: []array.Flag{flagDev},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.UnmapDisk(ctx, OptUnmapDisk{Dev: in.String(flagDev.Name)})
			},
		},
		{
			Path:  []string{"add", "masking"},
			Short: "present disks to hosts in batch mode",
			Flags: []array.Flag{flagData},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddMasking(ctx, []byte(in.String(flagData.Name)))
			},
		},
		{
			Path:  []string{"createpair"},
			Short: "add a SRDF pairing for the device",
			Flags: []array.Flag{flagPair, flagRDFG, flagInvalidate, flagSRDFMode, flagSRDFType},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return nil, t.CreatePair(ctx, OptCreatePair{
					Pair:       in.String(flagPair.Name),
					RDFG:       in.String(flagRDFG.Name),
					Invalidate: in.String(flagInvalidate.Name),
					SRDFMode:   in.String(flagSRDFMode.Name),
					SRDFType:   in.String(flagSRDFType.Name),
				})
			},
		},
		{
			Path:  []string{"deletepair"},
			Short: "delete a SRDF pairing for the device",
			Flags: []array.Flag{flagDev},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DeletePair(ctx, OptDeletePair{Dev: in.String(flagDev.Name)})
			},
		},
		{
			Path:  []string{"set", "mode"},
			Short: "set SRDF mode",
			Flags: []array.Flag{flagDev, flagSRDFMode},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return nil, t.SetSRDFMode(ctx, OptSetSRDFMode{
					Dev:      in.String(flagDev.Name),
					SRDFMode: in.String(flagSRDFMode.Name),
				})
			},
		},
		dumper([]string{"get", "pools"}, "get thin pools", func(ctx context.Context) (any, error) {
			return t.SymCfgPoolList(ctx)
		}),
		dumper([]string{"get", "sgs"}, "get storage groups", func(ctx context.Context) (any, error) {
			return t.SymSGList(ctx, "")
		}),
		dumper([]string{"get", "srps"}, "get SRP names", func(ctx context.Context) (any, error) {
			return t.SymCfgSRPList(ctx)
		}),
		dumper([]string{"get", "directors"}, "get directors", func(ctx context.Context) (any, error) {
			return t.SymCfgDirectorList(ctx, "all")
		}),
		dumper([]string{"get", "tdevs"}, "get thin devs", func(ctx context.Context) (any, error) {
			return t.SymDevList(ctx, "")
		}),
		dumper([]string{"get", "views"}, "get masking views", func(ctx context.Context) (any, error) {
			return t.SymAccessListViewDetail(ctx)
		}),
	}
}
