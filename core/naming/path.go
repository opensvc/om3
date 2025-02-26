package naming

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/xmap"
	"github.com/opensvc/om3/util/xstrings"
)

type (
	// Path represents an opensvc object path-like identifier. Ex: ns1/svc/svc1
	Path struct {
		// Name is the name part of the path
		Name string
		// Namespace is the namespace part of the path
		Namespace string
		// Kind is the kind part of the path
		Kind Kind
	}

	// Paths is a list of object paths.
	Paths []Path

	// M is a map indexed by path string representation.
	M map[string]interface{}

	// Metadata is the parsed representation of a path, used by api handlers to ease dumb clients access to individual path fields.
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Kind      Kind   `json:"kind"`
	}

	pather interface {
		Path() Path
	}
)

const (
	// Separator is the character separating a path's namespace, kind and name
	Separator = "/"
)

var (
	Cluster = Path{Name: "cluster", Namespace: "root", Kind: KindCcfg}

	// ErrInvalid is raised when the path allocator can not return a path
	// because one of the path element is not valid.
	ErrInvalid = errors.New("invalid path")

	forbiddenNames = append(
		KindStrings,
		"node",
	)
)

// NewPath allocates a new path type from its elements
func NewPath(namespace string, kind Kind, name string) (Path, error) {
	return NewPathFromStrings(namespace, kind.String(), name)
}

func NewPathFromStrings(namespace, kind, name string) (Path, error) {
	var path Path

	// letter casing checks
	if name != strings.ToLower(name) {
		return path, fmt.Errorf("%w: uppercase letters are not allowed in path name: %s", ErrInvalid, name)
	}
	if namespace != strings.ToLower(namespace) {
		return path, fmt.Errorf("%w: uppercase letters are not allowed in path namespace: %s", ErrInvalid, namespace)
	}

	// apply defaults
	if kind == "" {
		kind = "svc"
	}
	if namespace == "" {
		namespace = "root"
	}

	k := ParseKind(kind)
	switch k {
	case KindInvalid:
		return path, fmt.Errorf("%w: invalid kind %s", ErrInvalid, kind)
	case KindNscfg:
		name = "namespace"
	}

	if name == "" {
		return path, fmt.Errorf("%w: name is empty", ErrInvalid)
	}
	validatedName := strings.TrimLeft(name, "0123456789.") // trim the slice number from the validated name
	if !hostname.IsValid(validatedName) {
		return path, fmt.Errorf("%w: invalid name %s (rfc952)", ErrInvalid, name)
	}
	if !hostname.IsValid(namespace) {
		return path, fmt.Errorf("%w: invalid namespace %s (rfc952)", ErrInvalid, namespace)
	}
	for _, reserved := range forbiddenNames {
		if reserved == name {
			return path, fmt.Errorf("%w: reserved name '%s'", ErrInvalid, name)
		}
	}
	path.Namespace = namespace
	path.Name = name
	path.Kind = k
	return path, nil
}

// ScalerSliceIndex returns the <i> int from a scaler slice name like <i>.<scalerName>
// Return -1 if not a scaler slice.
func (t Path) ScalerSliceIndex() int {
	l := strings.SplitN(t.Name, ".", 2)
	if len(l) != 2 {
		return -1
	}
	if i, err := strconv.Atoi(l[0]); err != nil {
		return -1
	} else {
		return i
	}
}

func (t Path) FQN() string {
	var s string
	if t.Kind == KindInvalid {
		return ""
	}
	if t.Namespace == "" {
		s += "root" + Separator
	} else {
		s += t.Namespace + Separator
	}
	s += t.Kind.String() + Separator
	return s + t.Name
}

func (t Path) String() string {
	var s string
	if t.Kind == KindInvalid {
		return ""
	}
	if t.Namespace != "" && t.Namespace != "root" {
		s += t.Namespace + Separator
	}
	if (t.Kind != KindSvc && t.Kind != KindCcfg) || s != "" {
		s += t.Kind.String() + Separator
	}
	return s + t.Name
}

func (t Path) Equal(o Path) bool {
	if t.Namespace != o.Namespace || t.Kind != o.Kind || t.Name != o.Name {
		return false
	}
	return true
}

// ToMetadata returns the parsed representation of the path
func (t Path) ToMetadata() *Metadata {
	return &Metadata{
		Name:      t.Name,
		Namespace: t.Namespace,
		Kind:      t.Kind,
	}
}

func (t Path) IsZero() bool {
	return t.Name == "" && t.Namespace == "" && t.Kind == KindInvalid
}

// ParsePaths returns a new naming.Paths from a []string path list.
func ParsePaths(l ...string) (Paths, error) {
	var errs error
	paths := make(Paths, 0)
	for _, s := range l {
		if p, err := ParsePath(s); err != nil {
			errs = errors.Join(errs, err)
		} else {
			paths = append(paths, p)
		}
	}
	return paths, errs
}

// ParsePath returns a new path struct from a path string representation
func ParsePath(s string) (Path, error) {
	var (
		name      string
		namespace string
		kind      string
	)
	if s != strings.ToLower(s) {
		return Path{}, fmt.Errorf("%w: uppercase letters are not allowed", ErrInvalid)
	}
	l := strings.Split(s, Separator)
	switch len(l) {
	case 3:
		namespace = l[0]
		kind = l[1]
		name = l[2]
	case 2:
		switch l[1] {
		case "": // ex: ns1/
			namespace = l[0]
			kind = "nscfg"
			name = "namespace"
		default: // ex: cfg/c1
			namespace = "root"
			kind = l[0]
			name = l[1]
		}
	case 1:
		switch l[0] {
		case "cluster":
			namespace = "root"
			kind = "ccfg"
			name = l[0]
		default:
			namespace = "root"
			kind = "svc"
			name = l[0]
		}
	}
	return NewPathFromStrings(namespace, kind, name)
}

// MarshalText implements the json interface
func (t Path) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements the json interface
func (t *Path) UnmarshalText(b []byte) error {
	s := string(b)
	if p, err := ParsePath(s); err != nil {
		return err
	} else {
		*t = p
		return nil
	}
}

// Match returns true if the object matches the pattern, using a fnmatch
// matching algorithm with a few special cases to mask the root namespace
// tricks and the svc object kind as default.
//
// Trick:
// The 'f*' pattern matches all svc objects in the root namespace.
// The '*' pattern matches all svc objects in all namespaces.
func (t Path) Match(pattern string) bool {
	if pattern == "**" {
		return true
	}
	if pattern == "*" {
		return t.Kind == KindSvc
	}
	l := strings.Split(pattern, "/")
	f := 0
	if strings.Contains(pattern, "**") {
		s := t.FQN()
		if fnmatch.Match(pattern, s, f) {
			return true
		}
		return false
	}
	switch len(l) {
	case 1:
		s := t.FQN()
		if fnmatch.Match("root/svc/"+pattern, s, f) {
			return true
		}
	case 2:
		s := t.FQN()
		if fnmatch.Match("root/"+pattern, s, f) {
			return true
		}
	case 3:
		s := t.FQN()
		if fnmatch.Match(pattern, s, f) {
			return true
		}
	}
	return false
}

// Path implements the Pather interface
func (t Path) Path() Path {
	return t
}

func (t Paths) String() string {
	l := make([]string, len(t))
	for i, p := range t {
		l[i] = p.String()
	}
	return strings.Join(l, ",")
}

func (t Paths) Existing() Paths {
	l := make(Paths, 0)
	for _, p := range t {
		if p.Exists() {
			l = append(l, p)
		}
	}
	return l
}

func (t Paths) Filter(pattern string) Paths {
	l := make(Paths, 0)
	for _, p := range t {
		if p.Match(pattern) {
			l = append(l, p)
		}
	}
	return l
}

// StrMap converts Paths into a map indexed by path string representation.
// This format is useful for fast Has(string) bool functions.
func (t Paths) StrMap() M {
	m := make(M)
	for _, p := range t {
		m[p.String()] = nil
		m[p.FQN()] = nil
	}
	return m
}

// StrSlice converts Paths into a string slice.
// This format is useful to prepare api handlers parameters.
func (t Paths) StrSlice() []string {
	l := make([]string, len(t))
	for i, p := range t {
		l[i] = p.String()
	}
	return l
}

// Namespaces return the list of unique namespaces in Paths
func (t Paths) Namespaces() []string {
	m := make(map[string]interface{})
	for _, p := range t {
		m[p.Namespace] = nil
	}
	return xmap.Keys(m)
}

func (t M) HasPath(path Path) bool {
	_, ok := t[path.String()]
	return ok
}

func (t M) Has(s string) bool {
	_, ok := t[s]
	return ok
}

func (t Paths) Merge(other Paths) Paths {
	m := make(map[Path]interface{})
	l := make(Paths, 0)
	for _, p := range t {
		m[p] = nil
		l = append(l, p)
	}
	for _, p := range other {
		if _, ok := m[p]; !ok {
			l = append(l, p)
		}
	}
	return l
}

// VarDir returns the directory on the local filesystem where the object
// variable persistent data is stored as files.
func (t Path) VarDir() string {
	var s string
	switch t.Namespace {
	case "", "root":
		s = fmt.Sprintf("%s/%s/%s", rawconfig.Paths.Var, t.Kind, t.Name)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.VarNs, t)
	}
	return filepath.FromSlash(s)
}

// TmpDir returns the directory on the local filesystem where the object
// stores its temporary files.
func (t Path) TmpDir() string {
	var s string
	switch {
	case t.Namespace != "", t.Namespace != "root":
		s = fmt.Sprintf("%s/%s/%s", rawconfig.Paths.TmpNs, t.Namespace, t.Kind)
	case t.Kind == KindSvc, t.Kind == KindCcfg:
		s = fmt.Sprintf("%s", rawconfig.Paths.Tmp)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.Tmp, t.Kind)
	}
	return filepath.FromSlash(s)
}

// LogDir returns the directory on the local filesystem where the object
// stores its temporary files.
func (t Path) LogDir() string {
	var s string
	switch {
	case t.Namespace != "", t.Namespace != "root":
		s = fmt.Sprintf("%s/%s/%s", rawconfig.Paths.LogNs, t.Namespace, t.Kind)
	case t.Kind == KindSvc, t.Kind == KindCcfg:
		s = fmt.Sprintf("%s", rawconfig.Paths.Log)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.Log, t.Kind)
	}
	return filepath.FromSlash(s)
}

// LogFile returns the object log file path on the local filesystem.
func (t Path) LogFile() string {
	return filepath.Join(t.LogDir(), t.Name+".log")
}

// FrozenFile returns the path of the flag file blocking orchestrations and resource restart.
func (t Path) FrozenFile() string {
	return filepath.Join(t.VarDir(), "frozen")
}

// ConfigFile returns the object configuration file path on the local filesystem.
func (t Path) ConfigFile() string {
	s := t.String()
	if s == "" {
		return ""
	}
	switch t.Namespace {
	case "", "root":
		s = fmt.Sprintf("%s/%s.conf", rawconfig.Paths.Etc, s)
	default:
		s = fmt.Sprintf("%s/%s.conf", rawconfig.Paths.EtcNs, s)
	}
	return filepath.FromSlash(s)
}

// Exists returns true if the object configuration file exists.
func (t Path) Exists() bool {
	return file.Exists(t.ConfigFile())
}

// InstalledPaths returns a list of every object path with a locally installed configuration file.
func InstalledPaths() (Paths, error) {
	l := make(Paths, 0)
	matches := make([]string, 0)
	patterns := []string{
		fmt.Sprintf("%s/*.conf", rawconfig.Paths.Etc),       // root svc
		fmt.Sprintf("%s/*/*.conf", rawconfig.Paths.Etc),     // root other
		fmt.Sprintf("%s/*/*/*.conf", rawconfig.Paths.EtcNs), // namespaces
	}
	for _, pattern := range patterns {
		m, err := filepath.Glob(pattern)
		if err != nil {
			return l, err
		}
		matches = append(matches, m...)
	}
	replacements := []string{
		fmt.Sprintf("%s/", rawconfig.Paths.EtcNs),
		fmt.Sprintf("%s/", rawconfig.Paths.Etc),
	}
	envNamespace := env.Namespace()
	envKind := ParseKind(env.Kind())
	for _, ps := range matches {
		for _, r := range replacements {
			ps = strings.Replace(ps, r, "", 1)
			ps = strings.Replace(ps, r, "", 1)
		}
		ps = xstrings.TrimLast(ps, 5) // strip trailing .conf
		p, err := ParsePath(ps)
		if err != nil {
			continue
		}
		if envKind != KindInvalid && envKind != p.Kind {
			continue
		}
		if envNamespace != "" && envNamespace != p.Namespace {
			continue
		}
		l = append(l, p)
	}
	return l, nil
}

func PathOf(o any) Path {
	if p, ok := o.(pather); ok {
		return p.Path()
	}
	return Path{}
}
