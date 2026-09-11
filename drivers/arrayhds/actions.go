package arrayhds

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// The options this array needs and no other declares. Everything else comes
// from the catalogue in core/array.
//
// The names and the option sets are the ones v2 reads, so a command line
// written for the v2 agent, and the ones the collector writes, run here
// unchanged.
var (
	flagDevNum = array.Flag{
		Name:  "devnum",
		Usage: "the device number, as the array, the collector or a host writes it",
		Kind:  array.String,
	}
	flagPool = array.Flag{
		Name:  "pool",
		Usage: "the pool to create the disk into",
		Kind:  array.String,
	}
)

// Actions returns what this array answers to.
func (t *Array) Actions() []array.Action {
	return []array.Action{
		{
			Path:  []string{"add", "disk"},
			Short: "add a volume and map",
			Flags: []array.Flag{array.FlagName, array.FlagSize, flagPool, array.FlagMapping},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddDisk(ctx, OptAddDisk{
					Name: in.String(array.FlagName.Name),
					Pool: in.String(flagPool.Name),
					Size: in.String(array.FlagSize.Name),
					// v2 declares no lun for add_disk: the number is the one
					// the domains have free, chosen when the volume is mapped.
					LUN:      -1,
					Mappings: in.StringSlice(array.FlagMapping.Name),
				})
			},
		},
		{
			Path:  []string{"del", "disk"},
			Short: "unmap a volume and delete",
			Flags: []array.Flag{flagDevNum},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelDisk(ctx, in.String(flagDevNum.Name))
			},
		},
		{
			Path:  []string{"resize", "disk"},
			Short: "resize a volume",
			Flags: []array.Flag{flagDevNum, array.FlagSize},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.ResizeDisk(ctx, in.String(flagDevNum.Name), in.String(array.FlagSize.Name))
			},
		},
		{
			Path:  []string{"rename", "disk"},
			Short: "set the label of a volume",
			Flags: []array.Flag{flagDevNum, array.FlagName},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.RenameDisk(ctx, in.String(flagDevNum.Name), in.String(array.FlagName.Name))
			},
		},
		{
			Path:  []string{"add", "map"},
			Short: "map a volume",
			Flags: []array.Flag{flagDevNum, array.FlagMapping, array.FlagLUN},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.AddMap(ctx, OptAddMap{
					DevNum:   in.String(flagDevNum.Name),
					Mappings: in.StringSlice(array.FlagMapping.Name),
					LUN:      in.Int(array.FlagLUN.Name),
				})
			},
		},
		{
			Path:  []string{"del", "map"},
			Short: "unmap a volume",
			Flags: []array.Flag{flagDevNum, array.FlagMapping},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.DelMap(ctx, in.String(flagDevNum.Name), in.StringSlice(array.FlagMapping.Name))
			},
		},
		{
			Path:  []string{"list", "logicalunits"},
			Short: "list volumes",
			Flags: []array.Flag{flagDevNum},
			Run: func(ctx context.Context, in array.Input) (any, error) {
				return t.ListLogicalUnits(ctx, in.String(flagDevNum.Name))
			},
		},
	}
}

// Reports returns the sections of its configuration this array pushes to the
// collector.
//
// The section names are the ones v2 pushes for an hds array, because the
// collector reads them to know what it was handed.
func (t *Array) Reports() []array.Report {
	subtarget := func(name string) func(context.Context) (any, error) {
		return func(ctx context.Context) (any, error) {
			out, err := t.run(ctx, true, true, "GetStorageArray", "subtarget="+name)
			if err != nil {
				return nil, err
			}
			return out, nil
		}
	}
	return []array.Report{
		{
			Key: "array",
			Get: func(ctx context.Context) (any, error) {
				return t.run(ctx, true, true, "GetStorageArray")
			},
		},
		{
			Key: "lu",
			Get: func(ctx context.Context) (any, error) {
				return t.getLogicalUnits(ctx, "")
			},
		},
		{Key: "arraygroup", Get: subtarget("ArrayGroup")},
		{
			Key: "port",
			Get: func(ctx context.Context) (any, error) {
				return t.getPorts(ctx)
			},
		},
		{
			Key: "pool",
			Get: func(ctx context.Context) (any, error) {
				return t.getPools(ctx)
			},
		},
	}
}
