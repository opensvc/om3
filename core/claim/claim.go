// Package claim is a namespace's claim on a cluster resource.
//
// A namespace consumes things the cluster owns and its peers share: the space
// of a pool, the addresses of a network. A claim says which resource, and how
// much of it the namespace may take:
//
//	[claim#1]
//	type = pool
//	name = dirquota
//	limit = 250m
//
//	[claim#2]
//	type = network
//	name = backend2
//	limit = 10
//
// The limit is expressed in the unit of the resource, so reading it is left to
// the package that hands that resource out. What is common is where the claim
// is declared and how it is found, which is this package.
package claim

import (
	"strings"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
)

// Limit is the limit a namespace declares on a cluster resource, and whether
// it declares one at all.
//
// It is read from the namespace configuration on this node, and never over the
// api: taking a share of a cluster resource is a common thing to do and most
// namespaces claim nothing, so it must not wait on a daemon to find that out.
// A namespace configuration is present on every cluster node, so there is
// nothing to ask a peer for.
//
// A namespace whose configuration this node does not hold is read as claiming
// nothing.
func Limit(namespace, claimType, name string) (string, bool, error) {
	p := naming.Path{Namespace: namespace, Kind: naming.KindNscfg, Name: "namespace"}
	configFile := p.ConfigFile()
	if !file.Exists(configFile) {
		return "", false, nil
	}
	// The configuration file path is both the write target and the source to
	// read, so it is passed twice.
	cfg, err := xconfig.NewObject(configFile, configFile)
	if err != nil {
		return "", false, err
	}
	for _, section := range cfg.SectionStrings() {
		if !strings.HasPrefix(section, "claim#") {
			continue
		}
		if cfg.Get(key.New(section, "type")) != claimType {
			continue
		}
		if cfg.Get(key.New(section, "name")) != name {
			continue
		}
		limit := cfg.Get(key.New(section, "limit"))
		if limit == "" {
			// A claim naming no limit says the namespace uses the resource,
			// not that it is capped on it.
			return "", false, nil
		}
		return limit, true, nil
	}
	return "", false, nil
}
