package xconfig

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/cvaroqui/ini"
	"github.com/iancoleman/orderedmap"
)

// NewObject configures and returns a T instance pointer.
// The first argument is the path of the configuration file to write to.
//
// The path must be repeated as one of the following sources to read it.
// Accepted sources are []byte in ini format or a configuration file path
// containing ini formatted data.
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
		return nil, fmt.Errorf("load config sources error: %w", err)
	} else {
		t.file = f
	}
	return t, nil
}

func toIniSource(i any) (any, error) {
	cf := ini.Empty()
	doOption := func(section *ini.Section, option string, i any) {
		section.Key(option).SetValue(fmt.Sprint(i))
	}
	doSection := func(cf *ini.File, sectionTitle string, i any) error {
		section, err := cf.NewSection(sectionTitle)
		if err != nil {
			return err
		}
		switch sectionData := i.(type) {
		case map[string]string:
			for option, value := range sectionData {
				doOption(section, option, value)
			}
		case map[string]any:
			for option, value := range sectionData {
				doOption(section, option, value)
			}
		case *orderedmap.OrderedMap:
			for _, option := range sectionData.Keys() {
				value, _ := sectionData.Get(option)
				doOption(section, option, value)
			}
		case orderedmap.OrderedMap:
			for _, option := range sectionData.Keys() {
				value, _ := sectionData.Get(option)
				doOption(section, option, value)
			}
		}
		return nil
	}
	switch data := i.(type) {
	case []byte:
		return data, nil
	case string:
		return data, nil
	case map[string]map[string]any:
		for sectionTitle, sectionIntf := range data {
			if err := doSection(cf, sectionTitle, sectionIntf); err != nil {
				return nil, err
			}
		}
		b := bytes.NewBuffer([]byte{})
		cf.WriteTo(b)
		return b.Bytes(), nil
	case map[string]map[string]string:
		for sectionTitle, sectionIntf := range data {
			if err := doSection(cf, sectionTitle, sectionIntf); err != nil {
				return nil, err
			}
		}
		b := bytes.NewBuffer([]byte{})
		cf.WriteTo(b)
		return b.Bytes(), nil
	case *orderedmap.OrderedMap:
		for _, sectionTitle := range data.Keys() {
			sectionIntf, _ := data.Get(sectionTitle)
			if err := doSection(cf, sectionTitle, sectionIntf); err != nil {
				return nil, err
			}
		}
		b := bytes.NewBuffer([]byte{})
		cf.WriteTo(b)
		return b.Bytes(), nil
	case orderedmap.OrderedMap:
		for _, sectionTitle := range data.Keys() {
			sectionIntf, _ := data.Get(sectionTitle)
			if err := doSection(cf, sectionTitle, sectionIntf); err != nil {
				return nil, err
			}
		}
		b := bytes.NewBuffer([]byte{})
		cf.WriteTo(b)
		return b.Bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported WithConfigData() type: %s", reflect.TypeOf(data))
	}
}
