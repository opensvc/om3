package datarecv

import (
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
)

// SectionDatastores returns the datastores a resource section reads keys of,
// each once, by the values of its keywords get returns, as evaluated for a
// node: the install lines installing from one, the references of the
// environment keywords, and the deprecated configs, secrets and environment
// keywords reading a datastore. Their relative paths are of the namespace
// given.
//
// A line naming its datastores by a pattern of their name, as
// "./sec/haproxy-*", is returned as that pattern: it reads the datastores
// matching it when it runs, a datastore made later as well.
//
// The values are read from the configuration alone, whatever the state of
// the resource: what an object uses is what its configuration names, which
// a stopped instance starts with again.
func SectionDatastores(get func(option string) []string, namespace string) []string {
	l := make([]string, 0)
	add := func(s string) {
		if !slices.Contains(l, s) {
			l = append(l, s)
		}
	}
	for _, line := range Split(get("install")) {
		_, words := Pop(line)
		word, words := Pop(words)
		if word != "from" {
			continue
		}
		word, _ = Pop(words)
		if naming.IsPathPattern(word) {
			if p, err := parseStorePattern(word, namespace); err == nil {
				add(p.String())
			}
			continue
		}
		p, err := naming.ParsePathRel(word, namespace)
		if err != nil {
			continue
		}
		switch p.Kind {
		case naming.KindSec, naming.KindCfg:
			add(p.String())
		}
	}
	for option, kind := range map[string]naming.Kind{"configs": naming.KindCfg, "secrets": naming.KindSec} {
		for _, ref := range get(option) {
			from, _, _ := strings.Cut(ref, ":")
			if p, _, ok := referenceStore(from, kind, namespace); ok {
				add(p.String())
			}
		}
	}
	for _, option := range []string{"environment", "create_environment"} {
		paths, err := EnvironmentDatastores(get(option), namespace)
		if err != nil {
			// The resource refuses it at its action.
			continue
		}
		for _, p := range paths {
			add(p.String())
		}
	}
	for option, kind := range map[string]naming.Kind{
		"configs_environment":        naming.KindCfg,
		"secrets_environment":        naming.KindSec,
		"create_configs_environment": naming.KindCfg,
		"create_secrets_environment": naming.KindSec,
	} {
		for _, item := range get(option) {
			// "VAR=name/key" or "name/keypattern"
			if _, after, found := strings.Cut(item, "="); found {
				item = after
			}
			name, _, _ := strings.Cut(item, "/")
			if name == "" {
				continue
			}
			if p, err := naming.NewPath(namespace, kind, name); err == nil {
				add(p.String())
			}
		}
	}
	slices.Sort(l)
	return l
}
