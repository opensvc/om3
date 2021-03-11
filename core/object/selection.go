package object

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

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
		apiConfigured      bool
		api                client.API
		local              bool
		paths              []Path
		installed          []Path
		server             string
	}

	// Action describes an action to execute on the selected objects using.
	Action struct {
		Lock        bool
		LockTimeout time.Duration
		LockGroup   string
		Method      string
		MethodArgs  []interface{}
	}

	// ActionResult is a predictible type of actions return value, for reflect
	ActionResult struct {
		Nodename string      `json:"nodename"`
		Path     Path        `json:"path"`
		Data     interface{} `json:"data"`
		Error    error       `json:"error,omitempty"`
		Panic    interface{} `json:"panic,omitempty"`
	}
)

// NewSelection allocates a new object selection
func NewSelection(selector string) *Selection {
	t := &Selection{
		SelectorExpression: selector,
	}
	return t
}

// SetAPI sets the api struct key
func (t *Selection) SetAPI(api client.API) *Selection {
	t.api = api
	t.apiConfigured = true
	return t
}

// SetLocal sets the local struct key
func (t *Selection) SetLocal(local bool) *Selection {
	t.local = local
	return t
}

// SetServer sets the server struct key
func (t *Selection) SetServer(server string) *Selection {
	t.server = server
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
	log.Debugf("%d objects selected", len(t.paths))
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
	if !t.local {
		if !t.apiConfigured {
			c := client.NewConfig()
			c.SetURL(t.server)
			api, _ := c.NewAPI()
			t.SetAPI(api)
		}
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

// Installed returns the list of all paths with a locally installed
// configuration file.
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
	if !t.api.HasRequester() {
		return errors.New("api has no requester")
	}
	handle := t.api.NewGetObjectSelector()
	handle.ObjectSelector = t.SelectorExpression
	b, err := handle.Do()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &t.paths)
}

// Do executes in parallel the action on all selected objects supporting
// the action.
func (t *Selection) Do(action Action) []ActionResult {
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
		fn := reflect.ValueOf(obj).MethodByName(action.Method)
		if fn.Kind() == reflect.Invalid {
			log.Errorf("unsupported method %s on %s", action.Method, path)
			continue
		}
		fa := make([]reflect.Value, len(action.MethodArgs))
		for k, arg := range action.MethodArgs {
			fa[k] = reflect.ValueOf(arg)
		}
		go func(path Path) {
			defer func() {
				if r := recover(); r != nil {
					q <- ActionResult{
						Path:     path,
						Nodename: config.Node.Hostname,
						Panic:    r,
					}
				}
			}()
			values := fn.Call(fa)
			result := ActionResult{
				Path:     path,
				Nodename: config.Node.Hostname,
			}
			switch len(values) {
			case 0:
			case 1:
				result.Error, _ = values[0].Interface().(error)
			case 2:
				result.Data = values[0].Interface()
				result.Error, _ = values[1].Interface().(error)
			}
			q <- result
		}(path)
		started++
	}

	for i := 0; i < started; i++ {
		r := <-q
		results = append(results, r)
	}
	return results
}
