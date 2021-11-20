package object

import (
	"fmt"

	"opensvc.com/opensvc/core/driverid"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/util/key"
)

// OptsGet is the options of the Get function of all base objects.
type OptsDoc struct {
	Global  OptsGlobal
	Keyword string `flag:"kw"`
	Driver  string `flag:"driver"`
}

// Get returns a keyword value
func (t *Base) Doc(options OptsDoc) (string, error) {
	drvDoc := func(did driverid.T, kwName string) (string, error) {
		factory := resource.NewResourceFunc(did)
		if factory == nil {
			return "", fmt.Errorf("driver not found")
		}
		r := factory()
		buff := ""
		for _, kw := range r.Manifest().Keywords {
			if (kwName != "") && kw.Option != kwName {
				continue
			}
			buff += kw.Doc()
			buff += "\n"
		}
		return buff, nil
	}

	switch {
	case options.Driver != "":
		did := driverid.Parse(options.Driver)
		return drvDoc(*did, options.Keyword)
	case options.Keyword != "":
		k := key.Parse(options.Keyword)
		if k.Option != "" {
			return t.config.Doc(k)
		}
		sectionTypeKey := key.T{
			Section: k.Section,
			Option:  "type",
		}
		sectionType := t.config.Get(sectionTypeKey)
		rid, err := resourceid.Parse(k.Section)
		if err != nil {
			return "", err
		}
		did := driverid.T{
			Group: rid.DriverGroup(),
			Name:  sectionType,
		}
		return drvDoc(did, "")
	default:
		return "TODO", nil
	}
	return "", nil
}
