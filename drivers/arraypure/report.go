package arraypure

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// Reports returns the sections of its configuration this array pushes to the
// collector.
//
// The section names are the ones v2 pushes for a pure array, because the
// collector reads them to know what it was handed: a collector serving both
// agents stores what either of them pushes in the same place.
func (t *Array) Reports() []array.Report {
	return []array.Report{
		{
			Key: "arrays",
			Get: func(ctx context.Context) (any, error) {
				return t.GetArrays(ctx, OptGetItems{})
			},
		},
		{
			Key: "hardware",
			Get: func(ctx context.Context) (any, error) {
				return t.GetHardware(ctx, OptGetItems{})
			},
		},
		{
			Key: "pods",
			Get: func(ctx context.Context) (any, error) {
				return t.GetPods(ctx, OptGetItems{})
			},
		},
		{
			Key: "volumes",
			Get: func(ctx context.Context) (any, error) {
				return t.GetVolumes(ctx, OptGetItems{})
			},
		},
		{
			Key: "volumegroups",
			Get: func(ctx context.Context) (any, error) {
				return t.GetVolumeGroups(ctx, OptGetItems{})
			},
		},
		{
			Key: "ports",
			Get: func(ctx context.Context) (any, error) {
				return t.GetPorts(ctx, OptGetItems{})
			},
		},
	}
}
