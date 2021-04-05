package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/golang-collections/collections/set"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"opensvc.com/opensvc/core/path"
)

type (
	// T exposes methods to read and write configurations.
	T struct {
		ConfigFilePath string
		Path           path.T
		Dereferencer   Dereferencer
		v              *viper.Viper
		raw            Raw
	}

	// Dereferencer is the interface implemented by node and object to
	// provide a reference resolver using their private attributes.
	Dereferencer interface {
		Dereference(string) string
	}

	Raw map[string]interface{}
	Key string
)

var (
	RegexpReference = regexp.MustCompile(`({[\w\.-_:]+})`)
	RegexpOperation = regexp.MustCompile(`(\$\(\(.+\)\))`)
	RegexpScope     = regexp.MustCompile(`(@[\w\.-_]+)`)
	ErrorExists     = errors.New("does not exists")
)

func (t Key) section() string {
	l := strings.Split(string(t), ".")
	switch len(l) {
	case 2:
		return l[0]
	default:
		return "DEFAULT"
	}
}

func (t Key) option() string {
	l := strings.Split(string(t), ".")
	switch len(l) {
	case 2:
		return l[1]
	default:
		return l[0]
	}
}

func (t Key) scope() string {
	l := strings.Split(string(t), "@")
	switch len(l) {
	case 2:
		return l[1]
	default:
		return ""
	}
}

func (t T) Raw() Raw {
	return t.raw
}

//
// Get returns a key value,
// * contextualized for a node (by default the local node, customized by the
//   impersonate option)
// * dereferenced
// * evaluated
//
func (t *T) Get(key string) (interface{}, error) {
	val := t.v.Get(key)
	log.Debug().Msgf("config %s get %s => %s", t.ConfigFilePath, key, val)
	return val, nil
}

func (t *T) GetP(opts ...string) interface{} {
	key := strings.Join(opts, ".")
	return t.v.Get(key)
}

func (t *T) GetStringP(opts ...string) string {
	key := strings.Join(opts, ".")
	return t.v.GetString(key)
}

func (t *T) Eval(key string) (interface{}, error) {
	var (
		err error
		ok  bool
		s   interface{}
	)
	k := Key(key)
	section := k.section()
	option := k.option()
	s, ok = t.raw[section]
	if !ok {
		return nil, ErrorExists
	}
	if _, ok = s.(map[string]interface{}); !ok {
		return nil, ErrorExists
	}
	v, err := t.descope(s.(map[string]interface{}), option)
	if err != nil {
		return nil, err
	}
	var sv string
	if sv, ok = v.(string); !ok {
		return v, nil
	}
	sv = RegexpReference.ReplaceAllStringFunc(sv, func(ref string) string {
		return t.dereference(ref, section)
	})
	return sv, err
}

func (t *T) descope(s map[string]interface{}, option string) (interface{}, error) {
	if v, ok := s[option+"@"+Node.Hostname]; ok {
		return v, nil
	}
	if v, ok := s[option+"@nodes"]; ok && t.IsInNodes() {
		return v, nil
	}
	if v, ok := s[option+"@drpnodes"]; ok && t.IsInDRPNodes() {
		return v, nil
	}
	if v, ok := s[option+"@encapnodes"]; ok && t.IsInEncapNodes() {
		return v, nil
	}
	if v, ok := s[option]; ok {
		return v, nil
	}
	return nil, ErrorExists
}

func (t *T) Nodes() []string {
	l := t.v.GetStringSlice("default.nodes")
	if len(l) == 0 && os.Getenv("OSVC_CONTEXT") == "" {
		return []string{Node.Hostname}
	}
	return t.ExpandNodes(l)
}

func (t *T) DRPNodes() []string {
	l := t.v.GetStringSlice("default.drpnodes")
	return t.ExpandNodes(l)
}

func (t *T) EncapNodes() []string {
	l := t.v.GetStringSlice("default.encapnodes")
	return t.ExpandNodes(l)
}

func (t *T) ExpandNodes(nodes []string) []string {
	l := make([]string, 0)
	for _, n := range nodes {
		if strings.ContainsAny(n, "=") {
			l = append(l, t.NodesWithLabel(n)...)
		} else {
			l = append(l, n)
		}
	}
	return l
}

func (t *T) NodesWithLabel(label string) []string {
	l := make([]string, 0)
	/*
		e := strings.Split(label, "=")
		n := e[0]
		v := e[1]
	*/
	// TODO iterate nodes labels
	return l
}

func (t *T) IsInNodes() bool {
	s := set.New()
	for _, n := range t.Nodes() {
		s.Insert(n)
	}
	return s.Has(Node.Hostname)
}

func (t *T) IsInDRPNodes() bool {
	s := set.New()
	for _, n := range t.DRPNodes() {
		s.Insert(n)
	}
	return s.Has(Node.Hostname)
}

func (t *T) IsInEncapNodes() bool {
	s := set.New()
	for _, n := range t.EncapNodes() {
		s.Insert(n)
	}
	return s.Has(Node.Hostname)
}

func (t T) dereference(ref string, section string) string {
	switch ref {
	case "{rindex}":
		return strings.SplitN(section, "#", 2)[1]
	default:
		return t.Dereferencer.Dereference(ref)
	}
	return ref
}

func (t Raw) Render() string {
	s := ""
	for section, data := range t {
		if s == "metadata" {
			continue
		}
		s += Node.Colorize.Primary(fmt.Sprintf("[%s]\n", section))
		for k, v := range data.(map[string]interface{}) {
			if k == "comment" {
				s += renderComment(k, v)
				continue
			}
			s += renderKey(k, v)
		}
		s += "\n"
	}
	return s
}

func renderComment(k string, v interface{}) string {
	vs, ok := v.(string)
	if !ok {
		return ""
	}
	return "# " + strings.ReplaceAll(vs, "\n", "\n# ") + "\n"
}

func renderKey(k string, v interface{}) string {
	k = RegexpScope.ReplaceAllString(k, Node.Colorize.Error("$1"))
	vs, ok := v.(string)
	if ok {
		vs = RegexpReference.ReplaceAllString(vs, Node.Colorize.Optimal("$1"))
		vs = strings.ReplaceAll(vs, "\n", "\n\t")
	} else {
		vs = ""
	}
	return fmt.Sprintf("%s = %s\n", Node.Colorize.Secondary(k), vs)
}
