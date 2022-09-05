package xconfig

import (
	"path/filepath"

	"github.com/cvaroqui/ini"
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
