package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"unicode"

	"github.com/opensvc/om3/util/file"
)

type (
	Module struct {
		name    string
		path    string
		modset  string
		order   int
		autofix bool
	}
	Modules []*Module
)

var (
	reModuleStr = `^S*[0-9]+-*`
	reModule    = regexp.MustCompile(reModuleStr)
)

func (t Modules) Len() int      { return len(t) }
func (t Modules) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t Modules) Less(i, j int) bool {
	return t[i].Order() < t[j].Order()
}

func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("%s is a not executable (%s)", path, info.Mode())
	}
	uid, gid, err := file.Ownership(path)
	if err != nil {
		return err
	}
	if uid != 0 {
		return fmt.Errorf("%s is not owned by uid 0", path)
	}
	switch gid {
	case 0, 2, 3, 4:
	default:
		return fmt.Errorf("%s is not owned by gid 0,2,3,4", path)
	}
	return nil
}

func (t T) lookupModule(s string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(t.varDir, "*"+s))
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`^S*[0-9]+-*` + s + `$`)
	hits := make([]string, 0)
	for _, path := range paths {
		if !re.MatchString(filepath.Base(path)) {
			continue
		}
		locations := []string{path}
		if variants, err := filepath.Glob(filepath.Join(path, "main")); err == nil {
			locations = append(locations, variants...)
		}
		if variants, err := filepath.Glob(filepath.Join(path, "scripts", "main")); err == nil {
			locations = append(locations, variants...)
		}
		for _, path := range locations {
			if err := validatePath(path); err != nil {
				t.log.Tracef("%s discard: %s", path, err)
				continue
			}
			hits = append(hits, path)
		}
	}
	switch len(hits) {
	case 0:
		return "", fmt.Errorf("no installed modules found matching %s", s)
	case 1:
		return hits[0], nil
	default:
		return "", fmt.Errorf("multiple installed modules found matching %s: %s", s, hits)
	}
}

func parseModuleOrder(path string) int {
	base := filepath.Base(path)
	for i := 0; i < len(base); i++ {
		if !unicode.IsDigit(rune(base[i])) {
			n, _ := strconv.Atoi(base[:i])
			return n
		}
	}
	return -1
}

func NewModule(name string) *Module {
	mod := Module{
		name: name,
	}
	return &mod
}

func (t *T) NewValidModule(name string) (*Module, error) {
	mod := NewModule(name)
	if err := t.Validate(mod); err != nil {
		return nil, err
	}
	return mod, nil
}

func (t *T) Validate(mod *Module) error {
	path, err := t.lookupModule(mod.name)
	if err != nil {
		return err
	}
	mod.path = path
	mod.order = parseModuleOrder(path)
	return nil
}

func (t *Module) SetAutofix(v bool) *Module {
	t.autofix = v
	return t
}

func (t *Module) SetModulesetName(s string) *Module {
	t.modset = s
	return t
}

func (t Module) ModulesetName() string {
	return t.modset
}

func (t Module) Path() string {
	return t.path
}

func (t Module) Order() int {
	return t.order
}

func (t Module) Name() string {
	return t.name
}

func (t Module) Autofix() bool {
	return t.autofix
}

func (t T) ListModuleNames() ([]string, error) {
	mods, err := t.ListModules()
	if err != nil {
		return []string{}, err
	}
	l := make([]string, len(mods))
	for i, mod := range mods {
		l[i] = mod.Name()
	}
	return l, nil
}

func (t T) ListModules() (Modules, error) {
	l := make(Modules, 0)
	paths, err := filepath.Glob(filepath.Join(t.varDir, "*"))
	if err != nil {
		return l, nil
	}
	for _, path := range paths {
		base := filepath.Base(path)
		if !reModule.MatchString(base) {
			continue
		}
		name := reModule.ReplaceAllString(base, "")
		if mod, err := t.NewValidModule(name); err != nil {
			continue
		} else {
			l = append(l, mod)
		}
	}
	return l, nil
}
