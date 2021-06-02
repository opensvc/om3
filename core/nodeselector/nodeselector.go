package nodeselector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/golang-collections/collections/set"
	"github.com/rs/zerolog/log"
	"gopkg.in/errgo.v2/fmt/errors"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/xmap"
)

type (
	NodesInfo map[string]NodeInfo

	NodeInfo struct {
		Labels  map[string]string
		Targets interface{}
	}

	T struct {
		SelectorExpression string
		hasClient          bool
		client             *client.T
		local              bool
		nodes              []string
		server             string
		knownNodes         []string
		knownNodesSet      *set.Set
		info               NodesInfo
	}
)

var (
	fnmatchExpressionRegex = regexp.MustCompile(`[?*\[\]]`)
)

// New allocates a new node selector
func New(selector string, opts ...funcopt.O) *T {
	t := &T{
		SelectorExpression: selector,
	}
	_ = funcopt.Apply(t, opts...)
	return t
}

// WithClient sets the client struct key
func WithClient(client *client.T) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.client = client
		t.hasClient = true
		return nil
	})
}

// WithLocal forces the selection to be expanded without asking the
// daemon, which might result in an sub-selection of what the
// daemon would expand the selector to.
func WithLocal(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.local = v
		return nil
	})
}

// WithServer sets the server struct key
func WithServer(server string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.server = server
		return nil
	})
}

func (t T) String() string {
	return fmt.Sprintf("NodeSelector{%s}", t.SelectorExpression)
}

//
// LocalExpand resolves a selector expression into a list of object paths
// without asking the daemon for nodes information.
//
func LocalExpand(s string) []string {
	return New(s, WithLocal(true)).Expand()
}

//
// Expand resolves a selector expression into a list of object paths.
//
// First try to resolve using the daemon (remote or local), as the
// daemons know all cluster objects, even remote ones.
// If executed on a cluster node, fallback to a local selector, which
// looks up knownNodes configuration files.
//
func (t *T) Expand() []string {
	if t.nodes != nil {
		return t.nodes
	}
	if err := t.expand(); err != nil {
		log.Debug().Msg(err.Error())
		return t.nodes
	}
	log.Debug().Msgf("%d nodes selected", len(t.nodes))
	return t.nodes
}

//
// ExpandSet returns a set of the paths returned by Expand. Usually to
// benefit from the .Has() function.
//
func (t *T) ExpandSet() *set.Set {
	s := set.New()
	for _, p := range t.Expand() {
		s.Insert(p)
	}
	return s
}

func (t *T) add(node string) {
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
				log.Debug().Msgf("%s daemon expansion error: %s", t, err)
			}
		}
	}
	log.Debug().
		Str("selector", t.SelectorExpression).
		Str("mode", "local").
		Msg("expand selection")
	selector := t.SelectorExpression
	if selector == "" {
		selector = "*"
	}
	for _, s := range strings.Fields(selector) {
		pset, err := t.expandOne(s)
		if err != nil {
			return err
		}
		pset.Do(func(i interface{}) {
			if node, ok := i.(string); !ok {
				return
			} else {
				t.add(node)
			}
		})
	}
	return nil
}

func (t *T) expandOne(s string) (*set.Set, error) {
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

func (t *T) getKnownNodesSet() (*set.Set, error) {
	if t.knownNodesSet != nil {
		return t.knownNodesSet, nil
	}
	var err error
	t.knownNodes, err = t.KnownNodes()
	if err != nil {
		return t.knownNodesSet, err
	}
	t.knownNodesSet = set.New()
	for _, p := range t.knownNodes {
		t.knownNodesSet.Insert(p)
	}
	return t.knownNodesSet, nil
}

func (t *T) exactExpand(s string) (*set.Set, error) {
	matching := set.New()
	if hostname.IsValid(s) {
		return matching, errors.Newf("invalid hostname %s", s)
	}
	known, err := t.getKnownNodesSet()
	if err != nil {
		return nil, err
	}
	if !known.Has(s) {
		return matching, nil
	}
	matching.Insert(s)
	return matching, nil
}

func (t *T) fnmatchExpand(s string) (*set.Set, error) {
	matching := set.New()
	nodes, err := t.getKnownNodes()
	if err != nil {
		return matching, err
	}
	f := fnmatch.FNM_IGNORECASE | fnmatch.FNM_PATHNAME
	for _, node := range nodes {
		if fnmatch.Match(s, node, f) {
			matching.Insert(node)
		}
	}
	return matching, nil
}

func (t *T) labelExpand(s string) (*set.Set, error) {
	matching := set.New()
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
			matching.Insert(node)
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
	return strings.Fields(rawconfig.Node.Cluster.Nodes), nil
}

func (t T) daemonKnownNodes() ([]string, error) {
	if data, err := t.getNodesInfo(); err != nil {
		return []string{}, err
	} else {
		return xmap.Keys(data), nil
	}
}

func (t *T) getNodesInfo() (NodesInfo, error) {
	var err error
	if t.info != nil {
		return t.info, nil
	}
	if t.local {
		if t.info, err = t.getLocalNodesInfo(); err == nil {
			return t.info, nil
		}
		if t.info, err = t.getDaemonNodesInfo(); err == nil {
			return t.info, nil
		}
		return nil, err
	}
	if t.info, err = t.getDaemonNodesInfo(); err == nil {
		return t.info, nil
	} else if clientcontext.IsSet() {
		return nil, err
	}
	if t.info, err = t.getLocalNodesInfo(); err != nil {
		return nil, err
	}
	return t.info, nil
}

func (t T) getLocalNodesInfo() (NodesInfo, error) {
	var (
		err  error
		b    []byte
		data NodesInfo
	)
	p := filepath.Join(rawconfig.Node.Paths.Var, "nodes_info.json")
	log.Debug().Msgf("load %s", p)
	if b, err = ioutil.ReadFile(p); err != nil {
		return data, err
	}
	if err = json.Unmarshal(b, &data); err != nil {
		return data, err
	}
	return data, nil
}

func (t T) getDaemonNodesInfo() (NodesInfo, error) {
	data := make(NodesInfo)
	handle := t.client.NewGetNodesInfo()
	b, err := handle.Do()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &data)
	return data, err
}
