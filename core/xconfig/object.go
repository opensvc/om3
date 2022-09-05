package xconfig

import (
	"bytes"
	"path/filepath"
	"reflect"

	"github.com/cvaroqui/ini"
	"github.com/iancoleman/orderedmap"
	"github.com/pkg/errors"
)

//
// NewObject configures and returns a T instance pointer.
// The first argument is the path of the configuration file to write to.
//
// The path must be repeated as one of the following sources to read it.
// Accepted sources are []byte in ini format or a configuration file path
// containing ini formatted data.
//
func NewObject(p string, sources ...any) (*T, error) {
	t := &T{
		ConfigFilePath: filepath.FromSlash(p),
	}
	loadOptions := ini.LoadOptions{
		Loose:                      true,
		AllowPythonMultilineValues: true,
		SpaceBeforeInlineComment:   true,
	}
	for i, source := range sources {
		src, err := toIniSource(source)
		if err != nil {
			return nil, err
		}
		sources[i] = src
	}
	if len(sources) == 0 {
		sources = append(sources, []byte{})
	}
	if f, err := ini.LoadSources(loadOptions, sources[0], sources[1:]...); err != nil {
		return nil, errors.Wrap(err, "load config sources error")
	} else {
		t.file = f
	}
	return t, nil
}

func toIniSource(i any) (any, error) {
	cf := ini.Empty()
	switch data := i.(type) {
	case []byte:
		return data, nil
	case string:
		return data, nil
	case orderedmap.OrderedMap:
		for _, sectionTitle := range data.Keys() {
			sectionIntf, _ := data.Get(sectionTitle)
			section, err := cf.NewSection(sectionTitle)
			if err != nil {
				return nil, err
			}
			switch sectionData := sectionIntf.(type) {
			case orderedmap.OrderedMap:
				for _, option := range sectionData.Keys() {
					value, _ := sectionData.Get(option)
					section.Key(option).SetValue(value.(string))
				}
			}
		}
		b := bytes.NewBuffer([]byte{})
		cf.WriteTo(b)
		return b.Bytes(), nil
	default:
		return nil, errors.Errorf("unsupported WithConfigData() type: %s", reflect.TypeOf(data))
	}
}
