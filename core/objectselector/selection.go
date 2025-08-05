package objectselector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/goombaio/orderedset"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/xerrors"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/funcopt"
)

type (
	// Selection is the selection structure
	Selection struct {
		selectorExpression string
		client             *client.T
		local              bool

		// cache is the cached result of Expand()
		cache naming.Paths

		// paths is the list of path used by Expand() to expand paths from
		// selectorExpression
		paths naming.Paths

		pathSet *orderedset.OrderedSet

		isConfigFilterDisabled bool
		needCheckFilters       bool

		server string
	}
)

const (
	expressionNegationPrefix = "!"
)

var (
	fnmatchExpressionRegex = regexp.MustCompile(`[?*\[\]]`)
	configExpressionRegex  = regexp.MustCompile(`[=:><]`)
)

// New allocates a new object selection
func New(selector string, opts ...funcopt.O) *Selection {
	t := &Selection{
		selectorExpression: selector,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

// WithConfigFilterDisabled disable config filtering.
func WithConfigFilterDisabled() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.isConfigFilterDisabled = true
		// sets needCheckFilters to ensure Expand() calls CheckFilters().
		t.needCheckFilters = true
		return nil
	})
}

// WithClient sets the client struct key
func WithClient(client *client.T) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.client = client
		return nil
	})
}

// WithLocal forces the selection to be expanded without asking the
// daemon, which might result in an sub-selection of what the
// daemon would expand the selector to.
func WithLocal(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.local = v
		return nil
	})
}

// WithPaths forces a list of naming.Path from where the filtering
// will be done by Expand.
// The daemon knows the path of objects with no local instance, so better
// to use that instead of crawling etc/ via naming.InstalledPaths()
func WithPaths(paths naming.Paths) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Selection)
		t.paths = paths
		return nil
	})
}

func (t *Selection) String() string {
	return fmt.Sprintf("Selection{%s}", t.selectorExpression)
}

// Expand resolves a selector expression into a list of object paths.
//
// First try to resolve using the daemon (remote or local), as the
// daemons know all cluster objects, even remote ones.
// If executed on a cluster node, fallback to a local selector, which
// looks up installed configuration files.
func (t *Selection) Expand() (naming.Paths, error) {
	if t.cache != nil {
		return t.cache, nil
	}
	if t.needCheckFilters {
		if err := t.CheckFilters(); err != nil {
			return t.cache, err
		}
	}
	err := t.expand()
	return t.cache, err
}

// CheckFilters checks the filters
func (t *Selection) CheckFilters() error {
	err := t.checkFilters()
	if err == nil {
		t.needCheckFilters = false
	}
	return err
}

// checkFilters checks the filters
func (t *Selection) checkFilters() error {
	if !t.isConfigFilterDisabled {
		return nil
	}
	for _, s := range strings.Split(t.selectorExpression, ",") {
		if len(s) == 0 {
			continue
		}
		if configExpressionRegex.MatchString(s) {
			return fmt.Errorf("selection with config filter disabled can't use filter: '%s'", s)
		}
	}
	return nil
}

func (t *Selection) MustExpand() (naming.Paths, error) {
	if paths, err := t.Expand(); err != nil {
		return paths, err
	} else if len(paths) == 0 {
		return paths, fmt.Errorf("%s: %w", t.selectorExpression, xerrors.ObjectNotFound)
	} else {
		return paths, nil
	}
}

// SetPaths sets the paths from where the selection Expand is done.
func (t *Selection) SetPaths(installed naming.Paths) {
	t.paths = installed
	// we reset internal result cache to ensure next Expand evaluation
	t.cache = nil
}

func (t *Selection) add(p naming.Path) {
	pathStr := p.String()
	for _, e := range t.cache {
		if pathStr == e.String() {
			return
		}
	}
	t.cache = append(t.cache, p)
}

func (t *Selection) expand() error {
	var (
		err        error
		usedExpand string
	)
	t.cache = make(naming.Paths, 0)
	if t.local {
		usedExpand = "local"
		err = t.localExpand()
	} else {
		usedExpand = "daemon"
		err = t.daemonExpand()
		if err != nil && !clientcontext.IsSet() {
			usedExpand = "fallback local"
			err = t.localExpand()
		}
	}
	if err != nil {
		return fmt.Errorf("%s expand object selection: %w", usedExpand, err)
	}
	return nil
}

func (t *Selection) localExpand() error {
	for _, s := range strings.Split(t.selectorExpression, ",") {
		if len(s) == 0 {
			continue
		}
		pset, err := t.localExpandIntersector(s)
		if err != nil {
			return err
		}
		for _, i := range pset.Values() {
			p, _ := naming.ParsePath(i.(string))
			t.add(p)
		}
	}
	return nil
}

func (t *Selection) localExpandIntersector(s string) (*orderedset.OrderedSet, error) {
	pset := orderedset.NewOrderedSet()
	for i, selector := range strings.Split(s, "+") {
		ps, err := t.localExpandOne(selector)
		if err != nil {
			return pset, err
		}
		if i == 0 {
			for _, i := range ps.Values() {
				pset.Add(i)
			}
		} else {
			inter := orderedset.NewOrderedSet()
			for _, i := range ps.Values() {
				if pset.Contains(i) {
					inter.Add(i)
				}
			}
			pset = inter
		}
	}
	return pset, nil
}

func (t *Selection) localExpandOne(s string) (*orderedset.OrderedSet, error) {
	if strings.HasPrefix(s, expressionNegationPrefix) {
		return t.localExpandOneNegative(s)
	}
	return t.localExpandOnePositive(s)
}

func (t *Selection) localExpandOneNegative(s string) (*orderedset.OrderedSet, error) {
	var (
		positiveMatchSet *orderedset.OrderedSet
		pathSet          *orderedset.OrderedSet
		err              error
	)
	positiveExpression := strings.TrimLeft(s, expressionNegationPrefix)
	positiveMatchSet, err = t.localExpandOnePositive(positiveExpression)
	if err != nil {
		return orderedset.NewOrderedSet(), err
	}
	pathSet, err = t.getPathSet()
	if err != nil {
		return orderedset.NewOrderedSet(), err
	}
	negativeMatchSet := orderedset.NewOrderedSet()
	for _, i := range pathSet.Values() {
		if !positiveMatchSet.Contains(i) {
			negativeMatchSet.Add(i)
		}
	}
	return negativeMatchSet, nil
}

func (t *Selection) localExpandOnePositive(s string) (*orderedset.OrderedSet, error) {
	switch {
	case fnmatchExpressionRegex.MatchString(s):
		return t.localFnmatchExpand(s)
	case configExpressionRegex.MatchString(s):
		return t.localConfigExpand(s)
	default:
		return t.localExactExpand(s)
	}
}

// getPaths returns the list of paths set by the WithPaths() funcopt. If empty,
// scan the the local configuration directory for paths.
func (t *Selection) getPaths() (naming.Paths, error) {
	if t.paths != nil {
		return t.paths, nil
	}
	var err error
	t.paths, err = naming.InstalledPaths()
	if err != nil {
		return t.paths, err
	}
	return t.paths, nil
}

func (t *Selection) getPathSet() (*orderedset.OrderedSet, error) {
	if t.pathSet != nil {
		return t.pathSet, nil
	}
	paths, err := t.getPaths()
	if err != nil {
		return t.pathSet, err
	}
	t.pathSet = orderedset.NewOrderedSet()
	for _, path := range paths {
		t.pathSet.Add(path.String())
	}
	return t.pathSet, nil
}

func (t *Selection) localConfigExpand(s string) (*orderedset.OrderedSet, error) {
	matching := orderedset.NewOrderedSet()
	kop := keyop.Parse(s)
	paths, err := t.getPaths()
	if err != nil {
		return matching, err
	}
	for _, p := range paths {
		o, err := object.NewConfigurer(p, object.WithVolatile(true))
		if err != nil {
			return nil, err
		}
		if o.Config().HasKeyMatchingOp(*kop) {
			matching.Add(p.String())
			continue
		}
	}
	return matching, nil
}

func (t *Selection) localExactExpand(s string) (*orderedset.OrderedSet, error) {
	matching := orderedset.NewOrderedSet()
	path, err := naming.ParsePath(s)
	if err != nil {
		return matching, err
	}
	_, err = os.Stat(path.ConfigFile())
	if errors.Is(err, os.ErrNotExist) {
		return matching, xerrors.ObjectNotFound
	}
	if err != nil {
		return matching, err
	}
	matching.Add(s)
	return matching, nil
}

func (t *Selection) localFnmatchExpand(s string) (*orderedset.OrderedSet, error) {
	matching := orderedset.NewOrderedSet()
	paths, err := t.getPaths()
	if err != nil {
		return matching, err
	}
	for _, p := range paths {
		if p.Match(s) {
			matching.Add(p.String())
		}
	}
	return matching, nil
}

func (t *Selection) daemonExpand() error {
	if env.HasDaemonOrigin() {
		return fmt.Errorf("action origin is daemon")
	}
	if t.client == nil {
		return fmt.Errorf("no client defined")
	}
	params := api.GetObjectPathsParams{
		Path: t.selectorExpression,
	}

	if resp, err := t.client.GetObjectPaths(context.Background(), &params); err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected get objects selector status %s", resp.Status)
	} else {
		defer func() { _ = resp.Body.Close() }()
		return json.NewDecoder(resp.Body).Decode(&t.cache)
	}
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
		obj, err := object.New(p, opts...)
		if err != nil {
			return objs, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}
