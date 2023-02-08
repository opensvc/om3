package path

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/pkg/errors"

	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/xerrors"
	"github.com/opensvc/om3/util/xmap"
	"github.com/opensvc/om3/util/xstrings"
)

type (
	// T represents an opensvc object path-like identifier. Ex: ns1/svc/svc1
	T struct {
		// Name is the name part of the path
		Name string
		// Namespace is the namespace part of the path
		Namespace string
		// Kind is the kind part of the path
		Kind kind.T
	}

	// Relation is an object path or an instance path (path@node).
	Relation string

	// L is a list of object paths.
	L []T

	// M is a map indexed by path string representation.
	M map[string]interface{}

	// Metadata is the parsed representation of a path, used by api handlers to ease dumb clients access to individual path fields.
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Kind      kind.T `json:"kind"`
	}
)

const (
	// Separator is the character separating a path's namespace, kind and name
	Separator = "/"
)

var (
	Cluster = T{Name: "cluster", Namespace: "root", Kind: kind.Ccfg}

	// ErrInvalid is raised when the path allocator can not return a path
	// because one of the path element is not valid.
	ErrInvalid = errors.New("invalid path")

	forbiddenNames = append(
		kind.Names(),
		[]string{
			"node",
		}...,
	)
)

// New allocates a new path type from its elements
func New(name string, namespace string, kd string) (T, error) {
	var path T
	name = strings.ToLower(name)
	namespace = strings.ToLower(namespace)
	kd = strings.ToLower(kd)
	// apply defaults
	if kd == "" {
		kd = "svc"
	}
	if namespace == "" {
		namespace = "root"
	}

	k := kind.New(kd)
	switch k {
	case kind.Invalid:
		return path, errors.Wrapf(ErrInvalid, "invalid kind %s", kd)
	case kind.Nscfg:
		name = "namespace"
	}

	if name == "" {
		return path, errors.Wrap(ErrInvalid, "name is empty")
	}
	validatedName := strings.TrimLeft(name, "0123456789.") // trim the slice number from the validated name
	if !hostname.IsValid(validatedName) {
		return path, errors.Wrapf(ErrInvalid, "invalid name %s (rfc952)", name)
	}
	if !hostname.IsValid(namespace) {
		return path, errors.Wrapf(ErrInvalid, "invalid namespace %s (rfc952)", namespace)
	}
	for _, reserved := range forbiddenNames {
		if reserved == name {
			return path, errors.Wrapf(ErrInvalid, "reserved name '%s'", name)
		}
	}
	path.Namespace = namespace
	path.Name = name
	path.Kind = k
	return path, nil
}

// ScalerSliceIndex returns the <i> int from a scaler slice name like <i>.<scalerName>
// Return -1 if not a scaler slice.
func (t T) ScalerSliceIndex() int {
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

func (t T) FQN() string {
	var s string
	if t.Kind == kind.Invalid {
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

func (t T) String() string {
	var s string
	if t.Kind == kind.Invalid {
		return ""
	}
	if t.Namespace != "" && t.Namespace != "root" {
		s += t.Namespace + Separator
	}
	if (t.Kind != kind.Svc && t.Kind != kind.Ccfg) || s != "" {
		s += t.Kind.String() + Separator
	}
	return s + t.Name
}

func (t T) Equal(o T) bool {
	if t.Namespace != o.Namespace || t.Kind != o.Kind || t.Name != o.Name {
		return false
	}
	return true
}

// ToMetadata returns the parsed representation of the path
func (t *T) ToMetadata() *Metadata {
	return &Metadata{
		Name:      t.Name,
		Namespace: t.Namespace,
		Kind:      t.Kind,
	}
}

func (t T) IsZero() bool {
	return t.Name == "" && t.Namespace == "" && t.Kind == kind.Invalid
}

// ParseList returns a new path.L from a []string path list.
func ParseList(l ...string) (L, error) {
	var errs error
	paths := make(L, 0)
	for _, s := range l {
		if p, err := Parse(s); err != nil {
			xerrors.Append(errs, err)
		} else {
			paths = append(paths, p)
		}
	}
	return paths, errs
}

// Parse returns a new path struct from a path string representation
func Parse(s string) (T, error) {
	var (
		name      string
		namespace string
		kd        string
	)
	s = strings.ToLower(s)
	l := strings.Split(s, Separator)
	switch len(l) {
	case 3:
		namespace = l[0]
		kd = l[1]
		name = l[2]
	case 2:
		switch l[1] {
		case "": // ex: ns1/
			namespace = l[0]
			kd = "nscfg"
			name = "namespace"
		default: // ex: cfg/c1
			namespace = "root"
			kd = l[0]
			name = l[1]
		}
	case 1:
		switch l[0] {
		case "cluster":
			namespace = "root"
			kd = "ccfg"
			name = l[0]
		default:
			namespace = "root"
			kd = "svc"
			name = l[0]
		}
	}
	return New(name, namespace, kd)
}

// MarshalJSON implements the json interface
func (t T) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(t.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON implements the json interface
func (t *T) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := Parse(s)
	if err != nil {
		return err
	}
	t.Name = p.Name
	t.Namespace = p.Namespace
	t.Kind = p.Kind
	return nil
}

// Match returns true if the object matches the pattern, using a fnmatch
// matching algorithm with a few special cases to mask the root namespace
// tricks and the svc object kind as default.
//
// Trick:
// The 'f*' pattern matches all svc objects in the root namespace.
// The '*' pattern matches all svc objects in all namespaces.
func (t T) Match(pattern string) bool {
	if pattern == "**" {
		return true
	}
	l := strings.Split(pattern, "/")
	f := fnmatch.FNM_IGNORECASE
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
		if fnmatch.Match("*/svc/"+pattern, s, f) {
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
func (t T) Path() T {
	return t
}

func (t Relation) String() string {
	return string(t)
}

func (t Relation) Split() (T, string, error) {
	p, err := t.Path()
	return p, t.Node(), err
}

func (t Relation) Node() string {
	var s string
	if strings.Contains(string(t), "@") {
		s = strings.SplitN(string(t), "@", 1)[1]
	}
	return s
}

func (t Relation) Path() (T, error) {
	var s string
	if strings.Contains(string(t), "@") {
		s = strings.SplitN(string(t), "@", 1)[0]
	}
	s = string(t)
	return Parse(s)
}

func (t L) String() string {
	l := make([]string, len(t))
	for i, p := range t {
		l[i] = p.String()
	}
	return strings.Join(l, ",")
}

func (t L) Filter(pattern string) L {
	l := make(L, 0)
	for _, p := range t {
		if p.Match(pattern) {
			l = append(l, p)
		}
	}
	return l
}

// StrMap converts L into a map indexed by path string representation.
// This format is useful for fast Has(string) bool functions.
func (t L) StrMap() M {
	m := make(M)
	for _, p := range t {
		m[p.String()] = nil
	}
	return m
}

// Namespaces return the list of unique namespaces in L
func (t L) Namespaces() []string {
	m := make(map[string]interface{})
	for _, p := range t {
		m[p.Namespace] = nil
	}
	return xmap.Keys(m)
}

func (t M) Has(s string) bool {
	_, ok := t[s]
	return ok
}

func (t L) Merge(other L) L {
	m := make(map[T]interface{})
	l := make(L, 0)
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
func (t T) VarDir() string {
	var s string
	switch t.Namespace {
	case "", "root":
		s = fmt.Sprintf("%s/%s/%s", rawconfig.Paths.Var, t.Kind, t.Name)
	default:
		s = fmt.Sprintf("%s/namespaces/%s", rawconfig.Paths.Var, t)
	}
	return filepath.FromSlash(s)
}

// TmpDir returns the directory on the local filesystem where the object
// stores its temporary files.
func (t T) TmpDir() string {
	var s string
	switch {
	case t.Namespace != "", t.Namespace != "root":
		s = fmt.Sprintf("%s/namespaces/%s/%s", rawconfig.Paths.Tmp, t.Namespace, t.Kind)
	case t.Kind == kind.Svc, t.Kind == kind.Ccfg:
		s = fmt.Sprintf("%s", rawconfig.Paths.Tmp)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.Tmp, t.Kind)
	}
	return filepath.FromSlash(s)
}

// LogDir returns the directory on the local filesystem where the object
// stores its temporary files.
func (t T) LogDir() string {
	var s string
	switch {
	case t.Namespace != "", t.Namespace != "root":
		s = fmt.Sprintf("%s/namespaces/%s/%s", rawconfig.Paths.Log, t.Namespace, t.Kind)
	case t.Kind == kind.Svc, t.Kind == kind.Ccfg:
		s = fmt.Sprintf("%s", rawconfig.Paths.Log)
	default:
		s = fmt.Sprintf("%s/%s", rawconfig.Paths.Log, t.Kind)
	}
	return filepath.FromSlash(s)
}

// LogFile returns the object log file path on the local filesystem.
func (t T) LogFile() string {
	return filepath.Join(t.LogDir(), t.Name+".log")
}

// ConfigFile returns the object configuration file path on the local filesystem.
func (t T) ConfigFile() string {
	s := t.String()
	switch t.Namespace {
	case "", "root":
		s = fmt.Sprintf("%s/%s.conf", rawconfig.Paths.Etc, s)
	default:
		s = fmt.Sprintf("%s/%s.conf", rawconfig.Paths.EtcNs, s)
	}
	return filepath.FromSlash(s)
}

// Exists returns true if the object configuration file exists.
func (t T) Exists() bool {
	return file.Exists(t.ConfigFile())
}

// List returns a list of every object path with a locally installed configuration file.
func List() (L, error) {
	l := make(L, 0)
	matches := make([]string, 0)
	patterns := []string{
		fmt.Sprintf("%s/*.conf", rawconfig.Paths.Etc),                // root svc
		fmt.Sprintf("%s/*/*.conf", rawconfig.Paths.Etc),              // root other
		fmt.Sprintf("%s/namespaces/*/*/*.conf", rawconfig.Paths.Etc), // namespaces
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
	envKind := kind.New(env.Kind())
	for _, ps := range matches {
		for _, r := range replacements {
			ps = strings.Replace(ps, r, "", 1)
			ps = strings.Replace(ps, r, "", 1)
		}
		ps = xstrings.TrimLast(ps, 5) // strip trailing .conf
		p, err := Parse(ps)
		if err != nil {
			continue
		}
		if envKind != kind.Invalid && envKind != p.Kind {
			continue
		}
		if envNamespace != "" && envNamespace != p.Namespace {
			continue
		}
		l = append(l, p)
	}
	return l, nil
}
