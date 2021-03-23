package object

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang-collections/collections/set"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// Selection is the selection structure
	Selection struct {
		SelectorExpression string
		hasClient          bool
		client             *client.T
		local              bool
		paths              []Path
		installed          []Path
		server             string
	}

	// BaseAction describes common options of actions to execute on the selected objects or node.
	BaseAction struct {
		Lock        bool
		LockTimeout time.Duration
		LockGroup   string
		Action      string
	}

	// Action describes an action to execute on the selected objects.
	Action struct {
		BaseAction
		Run func(Path) (interface{}, error)
	}

	// ActionResult is a predictible type of actions return value, for reflect.
	ActionResult struct {
		Nodename      string        `json:"nodename"`
		Path          Path          `json:"path"`
		Data          interface{}   `json:"data"`
		Error         error         `json:"error,omitempty"`
		Panic         interface{}   `json:"panic,omitempty"`
		HumanRenderer func() string `json:"-"`
	}
)

var (
	fnmatchExpressionRegex = regexp.MustCompile(`[\?\*\[\]]`)
	configExpressionRegex  = regexp.MustCompile(`[=:><]`)
)

// NewSelection allocates a new object selection
func NewSelection(selector string) *Selection {
	t := &Selection{
		SelectorExpression: selector,
	}
	return t
}

// SetClient sets the client struct key
func (t *Selection) SetClient(client *client.T) *Selection {
	t.client = client
	t.hasClient = true
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
	log.Debug().Msgf("%d objects selected", len(t.paths))
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
		if !t.hasClient {
			c := client.New().SetURL(t.server)
			t.SetClient(c)
		}
		if err := t.daemonExpand(); err == nil {
			return
		} else if client.WantContext() {
			log.Debug().Msgf("%s daemon expansion error: %s", t, err)
			return
		}
	}
	if err := t.localExpand(); err != nil {
		log.Debug().Err(err).Msg("")
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
	log.Debug().Msgf("%s local expansion", t)
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
	switch {
	case fnmatchExpressionRegex.MatchString(s):
		return t.localFnmatchExpand(s)
	case configExpressionRegex.MatchString(s):
		return t.localConfigExpand(s)
	default:
		return t.localExactExpand(s)
	}
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

func (t *Selection) localConfigExpand(s string) (*set.Set, error) {
	matching := set.New()
	log.Warn().Msg("TODO: localConfigExpand")
	return matching, nil
}

func (t *Selection) localExactExpand(s string) (*set.Set, error) {
	matching := set.New()
	path, err := NewPathFromString(s)
	if err != nil {
		return matching, err
	}
	if !path.Exists() {
		return matching, nil
	}
	matching.Insert(path.String())
	return matching, nil
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
	log.Debug().Msgf("%s daemon expansion", t)
	if config.HasDaemonOrigin() {
		return errors.New("Action origin is daemon")
	}
	if !t.client.HasRequester() {
		return errors.New("client has no requester")
	}
	handle := t.client.NewGetObjectSelector()
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
		go func(path Path) {
			result := ActionResult{
				Path:     path,
				Nodename: config.Node.Hostname,
			}
			defer func() {
				if r := recover(); r != nil {
					result.Panic = r
					q <- result
				}
			}()
			data, err := action.Run(path)
			result.Data = data
			result.Error = err
			result.HumanRenderer = func() string {
				return data.(Renderer).Render()
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
