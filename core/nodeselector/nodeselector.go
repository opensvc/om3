package nodeselector

import (
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
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/xmap"
)

type (
	T struct {
		SelectorExpression string
		hasClient          bool
		client             *client.T
		local              bool
		nodes              []string
		server             string
		knownNodes         []string
		knownNodesSet      *orderedset.OrderedSet
		info               nodesinfo.NodesInfo
		log                zerolog.Logger
	}
)

var (
	fnmatchExpressionRegex = regexp.MustCompile(`[?*\[\]]`)
)

// New allocates a new node selector
func New(selector string, opts ...funcopt.O) *T {
	t := &T{
		SelectorExpression: selector,
		log:                log.Logger,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

// WithClient sets the client struct key
func WithClient(client *client.T) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.client = client
		t.hasClient = true
		return nil
	})
}

// WithClient sets the client struct key
func WithLogger(log zerolog.Logger) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

// WithLocal forces the selection to be expanded without asking the
// daemon, which might result in an sub-selection of what the
// daemon would expand the selector to.
func WithLocal(v bool) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.local = v
		return nil
	})
}

// WithNodesInfo allow in-daemon callers to bypass nodesinfo.Load and nodesinfo.Req
// as they can access the NodesInfo faster from the data bus.
func WithNodesInfo(v nodesinfo.NodesInfo) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.info = v
		return nil
	})
}

// WithServer sets the server struct key
func WithServer(server string) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.server = server
		return nil
	})
}

func (t T) String() string {
	return fmt.Sprintf("NodeSelector{%s}", t.SelectorExpression)
}

// LocalExpand resolves a selector expression into a list of object paths
// without asking the daemon for nodes information.
func LocalExpand(s string) []string {
	return New(s, WithLocal(true)).Expand()
}

// Expand resolves a selector expression into a list of object paths.
//
// First try to resolve using the daemon (remote or local), as the
// daemons know all cluster objects, even remote ones.
// If executed on a cluster node, fallback to a local selector, which
// looks up knownNodes configuration files.
func (t *T) Expand() []string {
	if t.nodes != nil {
		return t.nodes
	}
	if err := t.expand(); err != nil {
		t.log.Debug().Msg(err.Error())
		return t.nodes
	}
	t.log.Debug().
		Bool("local", t.local).
		Str("selector", t.SelectorExpression).
		Strs("result", t.nodes).
		Msg("expand node selector")
	return t.nodes
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

func (t *T) mustHaveClient() error {
	if t.hasClient {
		return nil
	}
	c, err := client.New(
		client.WithURL(t.server),
	)
	if err != nil {
		return err
	}
	t.client = c
	t.hasClient = true
	return nil
}

func (t *T) expand() error {
	if !t.local {
		if err := t.mustHaveClient(); err != nil {
			if clientcontext.IsSet() {
				return err
			} else {
				t.log.Debug().Msgf("%s daemon expansion error: %s", t, err)
			}
		}
	}
	selector := t.SelectorExpression
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
	if t.local {
		return t.localKnownNodes()
	}
	return t.daemonKnownNodes()
}

func (t T) localKnownNodes() ([]string, error) {
	l := strings.Fields(rawconfig.ClusterSection().Nodes)
	for i := 0; i > len(l); i++ {
		l[i] = strings.ToLower(l[i])
	}
	return l, nil
}

func (t T) daemonKnownNodes() ([]string, error) {
	if data, err := t.getNodesInfo(); err != nil {
		return []string{}, err
	} else {
		return xmap.Keys(data), nil
	}
}

func (t *T) getNodesInfo() (nodesinfo.NodesInfo, error) {
	var err error
	if t.info != nil {
		return t.info, nil
	}
	if t.local {
		if t.info, err = nodesinfo.Load(); err == nil {
			return t.info, nil
		}
		if t.client == nil {
			// no fallback possible
			return nil, err
		}
		if t.info, err = nodesinfo.ReqWithClient(t.client); err == nil {
			return t.info, nil
		}
		return nil, err
	}
	if t.info, err = nodesinfo.ReqWithClient(t.client); err == nil {
		return t.info, nil
	} else if clientcontext.IsSet() {
		return nil, err
	}
	if t.info, err = nodesinfo.Load(); err != nil {
		return nil, err
	}
	return t.info, nil
}
