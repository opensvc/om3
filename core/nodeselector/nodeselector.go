package nodeselector

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/goombaio/orderedset"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/errgo.v2/fmt/errors"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

type (
	T struct {
		selectorExpression string
		nodes              []string
		knownNodes         []string
		knownNodesSet      *orderedset.OrderedSet
		info               nodesinfo.M
		log                zerolog.Logger
		client             *client.T
	}

	ResultMap map[string]any
)

var (
	ErrClusterNodeCacheEmpty = fmt.Errorf("cluster nodes cache is empty (unreadable cluster config or not a cluster node)")

	fnmatchExpressionRegex = regexp.MustCompile(`[?*\[\]]`)
)

func (m ResultMap) Has(s string) bool {
	_, ok := m[s]
	return ok
}

// New allocates a new node selector
func New(selector string, opts ...funcopt.O) *T {
	t := &T{
		selectorExpression: selector,
		log:                log.Logger,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

// WithLogger sets the logger
func WithLogger(log zerolog.Logger) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

// WithNodesInfo allow in-daemon callers to bypass nodesinfo.Load
// as they can access the NodesInfo faster from the data bus.
func WithNodesInfo(v nodesinfo.M) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.info = v
		return nil
	})
}

// WithClient is the api client to use when a clientcontext is set.
func WithClient(v *client.T) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.client = v
		return nil
	})
}

func (t T) String() string {
	return fmt.Sprintf("NodeSelector{%s}", t.selectorExpression)
}

// Expand resolves a selector expression into a list of object paths
func Expand(s string) ([]string, error) {
	return New(s).Expand()
}

// Expand resolves a selector expression into a list of nodes.
//
// First try to resolve using the daemon (remote or local), as the
// daemons know all cluster objects, even remote ones.
// If executed on a cluster node, fallback to a local selector, which
// looks up knownNodes state files.
func (t *T) Expand() ([]string, error) {
	if t.nodes != nil {
		return t.nodes, nil
	}
	if err := t.expand(); err != nil {
		return nil, err
	}
	return t.nodes, nil
}

func (t *T) ExpandMap() (ResultMap, error) {
	l, err := t.Expand()
	if err != nil {
		return nil, err
	}
	m := make(ResultMap)
	for _, s := range l {
		m[s] = nil
	}
	return m, nil
}

func (t *T) add(node string) {
	node = strings.ToLower(node)
	for _, e := range t.nodes {
		if node == e {
			return
		}
	}
	t.nodes = append(t.nodes, node)
}

func (t *T) expand() error {
	selector := t.selectorExpression
	for _, s := range strings.Fields(selector) {
		pset, err := t.expandOne(s)
		if err != nil {
			return err
		}
		for _, i := range pset.Values() {
			if node, ok := i.(string); !ok {
				break
			} else {
				t.add(node)
			}
		}
	}
	return nil
}

func (t *T) expandOne(s string) (*orderedset.OrderedSet, error) {
	switch {
	case strings.Contains(s, "="):
		return t.labelExpand(s)
	case fnmatchExpressionRegex.MatchString(s):
		return t.fnmatchExpand(s)
	default:
		return t.exactExpand(s)
	}
}

func (t *T) getKnownNodes() ([]string, error) {
	if t.knownNodes != nil {
		return t.knownNodes, nil
	}
	var err error
	t.knownNodes, err = t.KnownNodes()
	if err != nil {
		return t.knownNodes, err
	}
	return t.knownNodes, nil
}

func (t *T) getKnownNodesSet() (*orderedset.OrderedSet, error) {
	if t.knownNodesSet != nil {
		return t.knownNodesSet, nil
	}
	var err error
	t.knownNodes, err = t.KnownNodes()
	if err != nil {
		return t.knownNodesSet, err
	}
	t.knownNodesSet = orderedset.NewOrderedSet()
	for _, p := range t.knownNodes {
		t.knownNodesSet.Add(p)
	}
	return t.knownNodesSet, nil
}

func (t *T) exactExpand(s string) (*orderedset.OrderedSet, error) {
	s = strings.ToLower(s)
	matching := orderedset.NewOrderedSet()
	if !hostname.IsValid(s) {
		return matching, errors.Newf("invalid hostname %s", s)
	}
	known, err := t.getKnownNodesSet()
	if err != nil {
		return nil, err
	}
	if !known.Contains(s) {
		return matching, nil
	}
	matching.Add(s)
	return matching, nil
}

func (t *T) fnmatchExpand(s string) (*orderedset.OrderedSet, error) {
	matching := orderedset.NewOrderedSet()
	nodes, err := t.getKnownNodes()
	if err != nil {
		return matching, err
	}
	f := fnmatch.FNM_IGNORECASE | fnmatch.FNM_PATHNAME
	for _, node := range nodes {
		if fnmatch.Match(s, node, f) {
			matching.Add(node)
		}
	}
	return matching, nil
}

func (t *T) labelExpand(s string) (*orderedset.OrderedSet, error) {
	matching := orderedset.NewOrderedSet()
	l := strings.SplitN(s, "=", 2)
	nodesInfo, err := t.getNodesInfo()
	if err != nil {
		return nil, err
	}
	for node, info := range nodesInfo {
		v, ok := info.Labels[l[0]]
		if !ok {
			continue
		}
		if v == l[1] {
			matching.Add(node)
			continue
		}
	}
	return matching, nil
}

func (t T) KnownNodes() ([]string, error) {
	if clientcontext.IsSet() || !cluster.ConfigData.IsSet() {
		return t.KnownRemoteNodes()
	} else {
		return t.KnownLocalNodes()
	}
}

func (t T) KnownRemoteNodes() ([]string, error) {
	var l []string
	nodesInfo, err := t.getNodesInfo()
	if err != nil {
		return nil, err
	}
	for node := range nodesInfo {
		l = append(l, node)
	}
	return l, nil
}

func (t T) KnownLocalNodes() ([]string, error) {
	l := cluster.ConfigData.Get().Nodes
	if len(l) == 0 {
		return l, ErrClusterNodeCacheEmpty
	}
	for i := 0; i > len(l); i++ {
		l[i] = strings.ToLower(l[i])
	}
	return l, nil
}

func (t *T) getNodesInfoFromAPI() (nodesinfo.M, error) {
	if t.client == nil {
		return nil, fmt.Errorf("no client")
	}
	resp, err := t.client.GetNodesInfoWithResponse(context.Background())
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return nodesinfo.M(*resp.JSON200), nil
	default:
		return nil, fmt.Errorf("%s", resp.Status())
	}
}

func (t *T) getNodesInfo() (nodesinfo.M, error) {
	if t.info != nil {
		return t.info, nil
	}
	if clientcontext.IsSet() {
		if info, err := t.getNodesInfoFromAPI(); err != nil {
			return nil, err
		} else {
			t.info = info
		}
	} else {
		if info, err := nodesinfo.Load(); err != nil {
			return nil, err
		} else {
			t.info = info
		}
	}
	return t.info, nil
}
