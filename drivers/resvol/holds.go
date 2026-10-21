package resvol

import (
	"strconv"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
)

// Holds says the volume object, of the namespace of the object: used
// exclusively, unless exclusive is false, its use then shared with the other
// consumers saying so.
func (t *T) Holds(src resource.HoldSource) []resource.Hold {
	name, ok := src.Get("name")
	if !ok || name == "" {
		return nil
	}
	p := naming.Path{Namespace: src.Path().Namespace, Kind: naming.KindVol, Name: name}
	class := resource.HoldVolume
	if s, ok := src.Get("exclusive"); ok {
		if exclusive, err := strconv.ParseBool(s); err == nil && !exclusive {
			class = resource.HoldSharedVolume
		}
	}
	return resource.HoldsOf(class, p.String())
}
