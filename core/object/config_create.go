package object

import (
	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/key"
)

var keyID = key.New("DEFAULT", "id")

// CreatedConfig returns the configuration b of an object created at p, as it
// is written, and the configurer reading it. The keyword operations sets
// apply last, so a keyword they name is kept, recorded or not: they say what
// the creator wants of this object.
//
// The DEFAULT section is given a header, as OpenSVC v2 wrote it, so the
// keywords of that section are grouped under it whatever their rank.
//
// Unless restore, the configuration is another object's copied, as a clone,
// a template or a configuration saved elsewhere: what it recorded of the thing
// it was made for does not come with it, the id it was created with, the disk
// its provision created, the address it drew. They are written again, for the
// new object, when it makes what they name. The id is given again at once, as
// an object has one from the moment it is created. With restore, the object is
// the one the configuration was saved from, and keeps them; it is given an id
// only when the configuration holds none.
func CreatedConfig(p naming.Path, b []byte, restore bool, sets ...keyop.T) (Configurer, []byte, error) {
	oc, err := NewConfigurer(p, WithConfigData(b), WithVolatile(true))
	if err != nil {
		return nil, nil, err
	}
	cfg := oc.Config()
	cfg.MaterializeDefaultSection()
	if !restore {
		cfg.UnsetRecorded()
	}
	if !restore || cfg.GetString(keyID) == "" {
		if err := cfg.PrepareSet(*keyop.New(keyID, keyop.Set, uuid.New().String(), 0)); err != nil {
			return nil, nil, err
		}
	}
	if err := cfg.PrepareSet(sets...); err != nil {
		return nil, nil, err
	}
	b, err = cfg.Dump()
	if err != nil {
		return nil, nil, err
	}
	return oc, b, nil
}
