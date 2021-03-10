package object

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/golang-collections/collections/set"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// Selection is the selection structure
	Selection struct {
		SelectorExpression string
		API                client.API
		Local              bool
		paths              []Path
		installed          []Path
	}
)

// NewSelection allocates a new object selection
func NewSelection(selector string) *Selection {
	t := &Selection{
		SelectorExpression: selector,
	}
	return t
}

// SetAPI sets the API struct key
func (t *Selection) SetAPI(api client.API) *Selection {
	t.API = api
	return t
}

// SetLocal sets the Local struct key
func (t *Selection) SetLocal(local bool) *Selection {
	t.Local = local
	return t
}

func (t Selection) String() string {
	return fmt.Sprintf("Selection{%s}", t.SelectorExpression)
}

//
// Expand resolves a selector expression into a list of object paths.
//
// First try to resolve using the daemon (remote or local), as the
// daemons know all cluster objects, even remote ones.
// If executed on a cluster node, fallback to a local selector, which
// looks up installed configuration files.
//
func (t *Selection) Expand() []Path {
	if t.paths != nil {
		return t.paths
	}
	t.expand()
	return t.paths
}

func (t *Selection) add(path Path) {
	pathStr := path.String()
	for _, p := range t.paths {
		if pathStr == p.String() {
			return
		}
	}
	t.paths = append(t.paths, path)
}

func (t *Selection) expand() {
	if !t.Local {
		if err := t.daemonExpand(); err == nil {
			return
		} else if client.WantContext() {
			log.Debugf("%s daemon expansion error: %s", t, err)
			return
		}
	}
	if err := t.localExpand(); err != nil {
		log.Debug(err)
	}
}

// Installed returns a list Path of every object with a locally installed configuration file.
func Installed() ([]Path, error) {
	l := make([]Path, 0)
	matches := make([]string, 0)
	patterns := []string{
		fmt.Sprintf("%s/*.conf", config.Node.Paths.Etc),                // root svc
		fmt.Sprintf("%s/*/*.conf", config.Node.Paths.Etc),              // root other
		fmt.Sprintf("%s/namespaces/*/*/*.conf", config.Node.Paths.Etc), // namespaces
	}
	for _, pattern := range patterns {
		m, err := filepath.Glob(pattern)
		if err != nil {
			return l, err
		}
		matches = append(matches, m...)
	}
	replacements := []string{
		fmt.Sprintf("%s/", config.Node.Paths.EtcNs),
		fmt.Sprintf("%s/", config.Node.Paths.Etc),
	}
	envNamespace := config.EnvNamespace()
	envKind := NewKind(config.EnvKind())
	for _, p := range matches {
		for _, r := range replacements {
			p = strings.Replace(p, r, "", 1)
			p = strings.Replace(p, r, "", 1)
		}
		p = xstrings.TrimLast(p, 5) // strip trailing .conf
		path, err := NewPathFromString(p)
		if err != nil {
			continue
		}
		if envKind != KindInvalid && envKind != path.Kind {
			continue
		}
		if envNamespace != "" && envNamespace != path.Namespace {
			continue
		}
		l = append(l, path)
	}
	return l, nil
}

func (t *Selection) localExpand() error {
	log.Debugf("%s local expansion", t)
	for _, s := range strings.Split(t.SelectorExpression, ",") {
		pset, err := t.localExpandIntersector(s)
		if err != nil {
			return err
		}
		pset.Do(func(i interface{}) {
			p, _ := NewPathFromString(i.(string))
			t.add(p)
		})
	}
	log.Debugf("%d objects selected", len(t.paths))
	return nil
}

func (t *Selection) localExpandIntersector(s string) (*set.Set, error) {
	pset := set.New()
	for i, selector := range strings.Split(s, "+") {
		ps, err := t.localExpandOne(selector)
		if err != nil {
			return pset, err
		}
		if i == 0 {
			pset = pset.Union(ps)
		} else {
			pset = pset.Intersection(ps)
		}
	}
	return pset, nil
}

func (t *Selection) localExpandOne(s string) (*set.Set, error) {
	// t.localConfigExpand()
	return t.localFnmatchExpand(s)
}

func (t *Selection) Installed() ([]Path, error) {
	if t.installed != nil {
		return t.installed, nil
	}
	var err error
	t.installed, err = Installed()
	if err != nil {
		return t.installed, err
	}
	return t.installed, nil
}

func (t *Selection) localFnmatchExpand(s string) (*set.Set, error) {
	matching := set.New()
	paths, err := t.Installed()
	if err != nil {
		return matching, err
	}
	for _, p := range paths {
		if p.Match(s) {
			matching.Insert(p.String())
		}
	}
	return matching, nil
}

func (t *Selection) daemonExpand() error {
	log.Debugf("%s daemon expansion", t)
	if config.HasDaemonOrigin() {
		return errors.New("Action origin is daemon")
	}
	if t.API.Requester == nil {
		return errors.New("API not set")
	}
	handle := t.API.NewGetObjectSelector()
	handle.ObjectSelector = t.SelectorExpression
	b, err := handle.Do()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &t.paths)
}

// Action executes in parallel the action on all selected objects supporting
// the action.
func (t *Selection) Action(action string, args ...interface{}) []ActionResult {
	t.Expand()
	q := make(chan ActionResult, len(t.paths))
	results := make([]ActionResult, 0)
	started := 0

	for _, path := range t.paths {
		obj := path.NewObject()
		if obj == nil {
			log.Debugf("skip action on %s: no object allocator", path)
			continue
		}
		fn := reflect.ValueOf(obj).MethodByName(action)
		fa := make([]reflect.Value, len(args))
		for k, arg := range args {
			fa[k] = reflect.ValueOf(arg)
		}
		go func(path Path) {
			defer func() {
				if r := recover(); r != nil {
					q <- ActionResult{
						Path:  path,
						Panic: r,
					}
				}
			}()
			q <- fn.Call(fa)[0].Interface().(ActionResult)
		}(path)
		started++
	}

	for i := 0; i < started; i++ {
		r := <-q
		results = append(results, r)
	}
	return results
}
