package object

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golang-collections/collections/set"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// Selection is the selection structure
	Selection struct {
		SelectorExpression string
		hasClient          bool
		client             *client.T
		local              bool
		paths              []path.T
		installed          []path.T
		installedSet       *set.Set
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
		Run func(path.T) (interface{}, error)
	}

	// ActionResult is a predictible type of actions return value, for reflect.
	ActionResult struct {
		Nodename      string        `json:"nodename"`
		Path          path.T        `json:"path"`
		Data          interface{}   `json:"data"`
		Error         error         `json:"error,omitempty"`
		Panic         interface{}   `json:"panic,omitempty"`
		HumanRenderer func() string `json:"-"`
	}
)

const (
	expressionNegationPrefix = "!"
)

var (
	fnmatchExpressionRegex = regexp.MustCompile(`[?*\[\]]`)
	configExpressionRegex  = regexp.MustCompile(`[=:><]`)
)

func defaultHumanRenderer(data interface{}) string {
	if data == nil {
		return ""
	}
	switch v := data.(type) {
	case Renderer:
		return v.Render()
	case fmt.Stringer:
		return v.String()
	case string:
		return v + "\n"
	case []string:
		s := ""
		for _, e := range v {
			s += e + "\n"
		}
		return s
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// NewSelection allocates a new object selection
func NewSelection(selector string, opts ...funcopt.O) *Selection {
	t := &Selection{
		SelectorExpression: selector,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

// SelectionWithClient sets the client struct key
func SelectionWithClient(client *client.T) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.client = client
		t.hasClient = true
		return nil
	})
}

// SelectionWithLocal forces the selection to be expanded without asking the
// daemon, which might result in an sub-selection of what the
// daemon would expand the selector to.
func SelectionWithLocal(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.local = v
		return nil
	})
}

// SelectionWithServer sets the server struct key
func SelectionWithServer(server string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.server = server
		return nil
	})
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
func (t *Selection) Expand() ([]path.T, error) {
	if t.paths != nil {
		return t.paths, nil
	}
	err := t.expand()
	log.Debug().Msgf("%d objects selected", len(t.paths))
	return t.paths, err
}

//
// ExpandSet returns a set of the paths returned by Expand. Usually to
// benefit from the .Has() function.
//
func (t *Selection) ExpandSet() (*set.Set, error) {
	s := set.New()
	paths, err := t.Expand()
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		s.Insert(p)
	}
	return s, nil
}

func (t *Selection) add(p path.T) {
	pathStr := p.String()
	for _, e := range t.paths {
		if pathStr == e.String() {
			return
		}
	}
	t.paths = append(t.paths, p)
}

func (t *Selection) expand() error {
	if !t.local {
		if !t.hasClient {
			c, _ := client.New(
				client.WithURL(t.server),
			)
			t.client = c
			t.hasClient = true
		}
		if err := t.daemonExpand(); err == nil {
			return nil
		} else if clientcontext.IsSet() {
			return errors.Wrapf(err, "daemon expansion fatal error")
		} else {
			log.Debug().Msgf("%s daemon expansion error: %s", t, err)
		}
	}
	return t.localExpand()
}

// Installed returns a list Path of every object with a locally installed configuration file.
func Installed() ([]path.T, error) {
	l := make([]path.T, 0)
	matches := make([]string, 0)
	patterns := []string{
		fmt.Sprintf("%s/*.conf", rawconfig.Node.Paths.Etc),                // root svc
		fmt.Sprintf("%s/*/*.conf", rawconfig.Node.Paths.Etc),              // root other
		fmt.Sprintf("%s/namespaces/*/*/*.conf", rawconfig.Node.Paths.Etc), // namespaces
	}
	for _, pattern := range patterns {
		m, err := filepath.Glob(pattern)
		if err != nil {
			return l, err
		}
		matches = append(matches, m...)
	}
	replacements := []string{
		fmt.Sprintf("%s/", rawconfig.Node.Paths.EtcNs),
		fmt.Sprintf("%s/", rawconfig.Node.Paths.Etc),
	}
	envNamespace := env.Namespace()
	envKind := kind.New(env.Kind())
	for _, ps := range matches {
		for _, r := range replacements {
			ps = strings.Replace(ps, r, "", 1)
			ps = strings.Replace(ps, r, "", 1)
		}
		ps = xstrings.TrimLast(ps, 5) // strip trailing .conf
		p, err := path.Parse(ps)
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

func (t *Selection) localExpand() error {
	log.Debug().
		Str("selector", t.SelectorExpression).
		Str("mode", "local").
		Msg("expand object selection")
	for _, s := range strings.Split(t.SelectorExpression, ",") {
		pset, err := t.localExpandIntersector(s)
		if err != nil {
			return err
		}
		pset.Do(func(i interface{}) {
			p, _ := path.Parse(i.(string))
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
	if strings.HasPrefix(s, expressionNegationPrefix) {
		return t.localExpandOneNegative(s)
	}
	return t.localExpandOnePositive(s)
}

func (t *Selection) localExpandOneNegative(s string) (*set.Set, error) {
	var (
		positiveMatchSet *set.Set
		installedSet     *set.Set
		err              error
	)
	positiveExpression := strings.TrimLeft(s, expressionNegationPrefix)
	positiveMatchSet, err = t.localExpandOnePositive(positiveExpression)
	if err != nil {
		return set.New(), err
	}
	installedSet, err = t.getInstalledSet()
	if err != nil {
		return set.New(), err
	}
	negativeMatchSet := installedSet.Difference(positiveMatchSet)
	return negativeMatchSet, nil
}

func (t *Selection) localExpandOnePositive(s string) (*set.Set, error) {
	switch {
	case fnmatchExpressionRegex.MatchString(s):
		return t.localFnmatchExpand(s)
	case configExpressionRegex.MatchString(s):
		return t.localConfigExpand(s)
	default:
		return t.localExactExpand(s)
	}
}

//
// getInstalled returns the list of all paths with a locally installed
// configuration file.
//
func (t *Selection) getInstalled() ([]path.T, error) {
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

func (t *Selection) getInstalledSet() (*set.Set, error) {
	if t.installedSet != nil {
		return t.installedSet, nil
	}
	var err error
	t.installed, err = Installed()
	if err != nil {
		return t.installedSet, err
	}
	t.installedSet = set.New()
	for _, p := range t.installed {
		t.installedSet.Insert(p.String())
	}
	return t.installedSet, nil
}

func (t *Selection) localConfigExpand(s string) (*set.Set, error) {
	matching := set.New()
	kop := keyop.Parse(s)
	paths, err := t.getInstalled()
	if err != nil {
		return matching, err
	}
	for _, p := range paths {
		o, err := NewConfigurerFromPath(p, WithVolatile(true))
		if err != nil {
			return nil, err
		}
		if o.Config().HasKeyMatchingOp(*kop) {
			matching.Insert(p.String())
			continue
		}
	}
	return matching, nil
}

func (t *Selection) localExactExpand(s string) (*set.Set, error) {
	matching := set.New()
	p, err := path.Parse(s)
	if err != nil {
		return matching, err
	}
	o, err := NewBaserFromPath(p)
	if err != nil {
		return nil, err
	}
	if !o.Exists() {
		return matching, nil
	}
	matching.Insert(p.String())
	return matching, nil
}

func (t *Selection) localFnmatchExpand(s string) (*set.Set, error) {
	matching := set.New()
	paths, err := t.getInstalled()
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
	log.Debug().
		Str("selector", t.SelectorExpression).
		Str("mode", "daemon").
		Msg("expand selection")
	if env.HasDaemonOrigin() {
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
func (t *Selection) Do(action Action) ([]ActionResult, error) {
	results := make([]ActionResult, 0)

	paths, err := t.Expand()
	if err != nil {
		return results, err
	}

	q := make(chan ActionResult, len(t.paths))
	started := 0

	for _, p := range paths {
		go func(p path.T) {
			result := ActionResult{
				Path:     p,
				Nodename: hostname.Hostname(),
			}
			defer func() {
				if r := recover(); r != nil {
					result.Panic = r
					fmt.Println(string(debug.Stack()))
					q <- result
				}
			}()
			data, err := action.Run(p)
			result.Data = data
			result.Error = err
			result.HumanRenderer = func() string { return defaultHumanRenderer(data) }
			q <- result
		}(p)
		started++
	}

	for i := 0; i < started; i++ {
		r := <-q
		results = append(results, r)
	}
	return results, nil
}

// Objects returns the selected list of objects. This function relays its
// funcopts to the object.NewFromPath() function.
func (t *Selection) Objects(opts ...funcopt.O) ([]interface{}, error) {
	objs := make([]interface{}, 0)

	paths, err := t.Expand()
	if err != nil {
		return objs, err
	}

	for _, p := range paths {
		obj, err := NewFromPath(p, opts...)
		if err != nil {
			return objs, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}
