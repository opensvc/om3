package object

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/fqdn"
	"opensvc.com/opensvc/util/key"
)

var (
	RegexpScalerPrefix = regexp.MustCompile(`^[0-9]+\.`)
)

func (t *Base) loadConfig() error {
	var err error
	if t.config, err = config.NewObject(t.ConfigFile()); err != nil {
		return err
	}
	t.config.Path = t.Path
	t.config.Referrer = t
	return err
}

func (t Base) Config() *config.T {
	return t.config
}

func (t Base) ID() uuid.UUID {
	if t.id != uuid.Nil {
		return t.id
	}
	idKey := key.Parse("id")
	if idStr := t.config.GetString(idKey); idStr != "" {
		if id, err := uuid.Parse(idStr); err == nil {
			t.id = id
			return t.id
		}
	}
	t.id = uuid.New()
	_ = t.config.Set(idKey, t.id.String())
	if err := t.config.Commit(); err != nil {
		t.log.Error().Err(err).Msg("")
	}
	return t.id
}

func (t Base) Dereference(ref string) string {
	switch ref {
	case "id":
		return t.ID().String()
	case "name", "{svcname}":
		return t.Path.Name
	case "short_name", "{short_svcname}":
		return strings.SplitN(t.Path.Name, ".", 1)[0]
	case "scaler_name", "{scaler_svcname}":
		return RegexpScalerPrefix.ReplaceAllString(t.Path.Name, "")
	case "scaler_short_name", "{scaler_short_svcname}":
		return strings.SplitN(RegexpScalerPrefix.ReplaceAllString(t.Path.Name, ""), ".", 1)[0]
	case "namespace":
		return t.Path.Namespace
	case "kind":
		return t.Path.Kind.String()
	case "path", "{svcpath}":
		if t.Path.IsZero() {
			return ""
		}
		return t.Path.String()
	case "fqdn":
		if t.Path.IsZero() {
			return ""
		}
		return fqdn.New(t.Path, config.Node.Cluster.Name).String()
	case "domain":
		if t.Path.IsZero() {
			return ""
		}
		return fqdn.New(t.Path, config.Node.Cluster.Name).Domain()
	case "private_var":
		return t.paths.varDir
	case "initd":
		return filepath.Join(filepath.Dir(t.ConfigFile()), t.Path.Name+".d")
	case "collector_api":
		return "TODO"
	case "clusterid":
		return "TODO"
	case "clustername":
		return "TODO"
	case "clusternodes":
		return "TODO"
	case "clusterdrpnodes":
		return "TODO"
	case "dns":
		return "TODO"
	case "dnsnodes":
		return "TODO"
	case "dnsuxsock":
		return t.Node().DNSUDSFile()
	case "dnsuxsockd":
		return t.Node().DNSUDSDir()
	}
	switch {
	case strings.HasPrefix(ref, "safe://"):
		return "TODO"
	}
	return ref
}

func (t Base) PostCommit() error {
	return nil
}
