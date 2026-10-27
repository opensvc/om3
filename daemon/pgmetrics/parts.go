package pgmetrics

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/metricsreg"
)

// partsCollector reports the subsets and the resources of the objects a
// scrape asks for, which the walk leaves out: a node runs about a thousand
// resources, each a set of series.
//
// Their groups are told from the configuration of the object, the process
// group of each resource and subset as om creates them, rather than from the
// names of the slices, whose escaping can not be undone.
type partsCollector struct {
	paths naming.Paths
}

func (c partsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range detailDescs {
		ch <- desc
	}
}

func (c partsCollector) Collect(ch chan<- prometheus.Metric) {
	var e emitter
	for _, p := range c.paths {
		collectParts(&e, p)
	}
	for _, metric := range e.out {
		ch <- metric
	}
}

// collectParts emits the series of the subsets and the resources of the
// object p, a resource in a subset naming it by its subset label. An object
// with no process group, or not installed on the node, has none.
func collectParts(e *emitter, p naming.Path) {
	if !hasCgroup(p.Kind) || !p.Exists() {
		return
	}
	o, err := object.NewActor(p, object.WithVolatile(true))
	if err != nil {
		return
	}
	namespace, path := p.Namespace, p.String()
	visit := func(cgroupPath string, labels ...string) {
		if !exists(cgroupPath) {
			e.gauge(cgroupExistsDesc, 0, labels...)
			return
		}
		e.gauge(cgroupExistsDesc, 1, labels...)
		collectCgroupMetrics(e, true, cgroupPath, labels...)
	}
	// The resources are configured, and their process groups set, on the
	// first listing.
	resources := o.Resources()
	done := make(map[string]bool)
	for _, rset := range o.ResourceSets() {
		if rset.Name == "" || rset.PG == nil || rset.PG.ID == "" || done[rset.PG.ID] {
			continue
		}
		// Two driver groups may have a subset of the same name, which
		// is one group.
		done[rset.PG.ID] = true
		visit(filepath.Join(HostCgroupRoot, rset.PG.ID), scopeSubset, namespace, path, rset.Name, "")
	}
	for _, r := range resources {
		pgPath := r.GetPGID()
		if p, ok := r.(resource.PGPather); ok {
			pgPath = p.PGPath()
		}
		if pgPath == "" {
			continue
		}
		visit(filepath.Join(HostCgroupRoot, pgPath), scopeResource, namespace, path, r.RSubset(), r.RID())
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Handler serves /metrics/pg: the series of the walk, and, for each path
// query parameter, the series of the subsets and the resources of that
// object.
func Handler() http.Handler {
	opts := promhttp.HandlerOpts{}
	walk := promhttp.HandlerFor(metricsreg.PG, opts)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := r.URL.Query()["path"]
		if len(l) == 0 {
			walk.ServeHTTP(w, r)
			return
		}
		paths, err := naming.ParsePaths(l...)
		if err != nil {
			http.Error(w, fmt.Sprintf("path: %s", err), http.StatusBadRequest)
			return
		}
		parts := prometheus.NewRegistry()
		parts.MustRegister(partsCollector{paths: paths})
		promhttp.HandlerFor(prometheus.Gatherers{metricsreg.PG, parts}, opts).ServeHTTP(w, r)
	})
}
