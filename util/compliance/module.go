package compliance

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"unicode"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/file"
)

type (
	Module struct {
		path string
		main *T
	}
)

var (
	reModuleStr = `^S*[0-9]+-*`
	reModule    = regexp.MustCompile(reModuleStr)
)

func (t *T) NewModule(path string) *Module {
	mod := Module{
		path: path,
		main: t,
	}
	return &mod
}

func (t Module) Validate() error {
	if t.Name() == t.Base() {
		return errors.Errorf("%s invalid filename: must match the %s regexp", t.Base(), reModuleStr)
	}
	info, err := os.Stat(t.path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.Errorf("%s is not a regular file", t.path)
	}
	if info.Mode()&0111 == 0 {
		return errors.Errorf("%s is a not executable (%s)", t.path, info.Mode())
	}
	uid, gid, err := file.Ownership(t.path)
	if err != nil {
		return err
	}
	if uid != 0 {
		return errors.Errorf("%s is not owned by uid 0", t.path)
	}
	switch gid {
	case 0, 2, 3, 4:
	default:
		return errors.Errorf("%s is not owned by gid 0,2,3,4", t.path)
	}
	return nil
}

func (t Module) Order() int {
	s := t.Base()
	for i := 0; i < len(s); i += 1 {
		if !unicode.IsDigit(rune(s[i])) {
			n, _ := strconv.Atoi(s[:i])
			return n
		}
	}
	return -1
}

func (t Module) Base() string {
	return filepath.Base(t.path)
}

func (t Module) Name() string {
	return reModule.ReplaceAllLiteralString(t.Base(), "")
}

func (t T) ListModules() ([]string, error) {
	l := make([]string, 0)
	paths, err := filepath.Glob(filepath.Join(t.varDir, "*"))
	if err != nil {
		return l, nil
	}
	for _, path := range paths {
		mod := t.NewModule(path)
		if err := mod.Validate(); err != nil {
			t.log.Debug().Msgf("discard module %s: %s", path, err)
			continue
		}
		l = append(l, mod.Name())
	}
	return l, nil
}
