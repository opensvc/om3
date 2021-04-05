package object

import (
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/fqdn"
)

func (t *Base) loadConfig() error {
	var err error
	t.config, err = config.NewObject(t.ConfigFile())
	t.config.Path = t.Path
	t.config.Dereferencer = *t
	return err
}

func (t Base) Config() *config.T {
	return t.config
}

func (t Base) Dereference(ref string) string {
	switch ref {
	case "{name}":
		return t.Path.Name
	case "{namespace}":
		return t.Path.Namespace
	case "{kind}":
		return t.Path.Kind.String()
	case "{path}", "{svcname}":
		if t.Path.IsZero() {
			return ""
		}
		return t.Path.String()
	case "{fqdn}":
		if t.Path.IsZero() {
			return ""
		}
		return fqdn.New(t.Path, config.Node.Cluster.Name).String()
	}
	return ref
}
