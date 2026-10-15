package resvol

import (
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the volume object, of the namespace of the object.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	name, ok := src.Get("name")
	if !ok || name == "" {
		return nil
	}
	p := naming.Path{Namespace: src.Path().Namespace, Kind: naming.KindVol, Name: name}
	return resource.HoldsOf(resource.HoldVolume, p.String())
}
