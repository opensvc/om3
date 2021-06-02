package nodeselector

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/golang-collections/collections/set"
	"github.com/rs/zerolog/log"
	"gopkg.in/errgo.v2/fmt/errors"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/core/env"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/xmap"
)

type T struct {
	SelectorExpression string
	hasClient          bool
	client             *client.T
	local              bool
	nodes              []string
	server             string
	knownNodes         []string
	knownNodesSet      *set.Set
}

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
	t.expand()
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

func (t *T) expand() {
	if !t.local {
		if !t.hasClient {
			c, _ := client.New(
				client.WithURL(t.server),
			)
			t.client = c
			t.hasClient = true
		}
		if err := t.daemonExpand(); err == nil {
			return
		} else if clientcontext.IsSet() {
			log.Debug().Msgf("%s daemon expansion error: %s", t, err)
			return
		} else {
			log.Debug().Msgf("%s daemon expansion error: %s", t, err)
		}
	}
	if err := t.localExpand(); err != nil {
		log.Debug().Err(err).Msg("")
	}
}

func (t *T) localExpand() error {
	log.Debug().
		Str("selector", t.SelectorExpression).
		Str("mode", "local").
		Msg("expand selection")
	for _, s := range strings.Fields(t.SelectorExpression) {
		pset, err := t.localExpandOne(s)
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

func (t *T) localExpandOne(s string) (*set.Set, error) {
	switch {
	case strings.Contains(s, "="):
		return t.localLabelExpand(s)
	case fnmatchExpressionRegex.MatchString(s):
		return t.localFnmatchExpand(s)
	default:
		return t.localExactExpand(s)
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

func (t *T) localExactExpand(s string) (*set.Set, error) {
	matching := set.New()
	if hostname.IsValid(s) {
		return matching, errors.Newf("invalid hostname %s", s)
	}
	if !t.knownNodesSet.Has(s) {
		return matching, nil
	}
	matching.Insert(s)
	return matching, nil
}

func (t *T) localFnmatchExpand(s string) (*set.Set, error) {
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

func (t *T) localLabelExpand(s string) (*set.Set, error) {
	matching := set.New()
	return matching, nil
}

func (t *T) daemonExpand() error {
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
	return nil
}

type (
	NodesInfo map[string]NodeInfo

	NodeInfo struct {
		Labels  map[string]string
		Targets interface{}
	}
)

func (t T) KnownNodes() ([]string, error) {
	if data, err := t.nodesInfo(); err != nil {
		return []string{hostname.Hostname()}, err
	} else {
		return xmap.Keys(data), nil
	}
}

func (t T) nodesInfo() (NodesInfo, error) {
	data := make(NodesInfo)
	handle := t.client.NewGetNodesInfo()
	b, err := handle.Do()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &data)
	return data, err
}
