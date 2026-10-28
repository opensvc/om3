// Package drivercatalog describes the drivers of the node: what
// each does, in a line, the features it has, named in a vocabulary of its
// own, the drivers a virtual driver selects among, and whether the node has
// what the driver needs.
//
// The features are words of their own rather than the names of the
// interfaces of the resource package they are told from: those are code,
// and a refactor renaming one would break a client keying on it.
package drivercatalog

import (
	"context"
	"syscall"

	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/capabilities"
)

type (
	// Feature is a word of the vocabulary saying what a driver does.
	Feature string

	// FeatureDoc is a feature and what it means.
	FeatureDoc struct {
		Name Feature
		Doc  string
	}

	// Entry is a driver of the catalog.
	Entry struct {
		ID driver.ID

		// Doc is the line saying what the driver does.
		Doc string

		// AliasOf is the driver this one is another name of, kept
		// for the configurations written with it.
		AliasOf driver.ID

		// Selects are the drivers a virtual driver picks among on a
		// node, in the order it prefers them.
		Selects driver.IDs

		// Features are what the driver does, in the vocabulary of
		// Features. A virtual driver has the ones of the driver it
		// selects on the node.
		Features []Feature

		// Available says the node has what the driver needs, as its
		// capabilities say.
		Available bool

		// Grant is the grant a service needs to use the driver at all,
		// empty when any user granted on the namespace may: the one
		// choosing the driver needs, or the one a keyword the driver
		// requires needs whatever its value.
		Grant string

		// ProvisionGrant is the grant only the provision of the
		// resource needs, beyond Grant: the one a keyword the
		// provision fails without needs whatever its value. A user not
		// holding it may still use the driver on what a sysadmin
		// provisioned, with provision = false.
		ProvisionGrant string
	}
)

const (
	FeatureProvision   Feature = "provision"
	FeatureUnprovision Feature = "unprovision"
	FeatureStart       Feature = "start"
	FeatureStop        Feature = "stop"
	FeatureRun         Feature = "run"
	FeatureSync        Feature = "sync"
	FeatureSnapshot    Feature = "snapshot"
	FeatureResize      Feature = "resize"
	FeatureSignal      Feature = "signal"
	FeatureAbort       Feature = "abort"
	FeatureMove        Feature = "move"
	FeatureEncap       Feature = "encap"
)

// Features is the vocabulary, in the order the features of a driver are
// listed in.
var Features = []FeatureDoc{
	{FeatureProvision, "The provision makes what the resource stands for, as a logical volume or a filesystem, and the resource can be configured on one made by a sysadmin instead."},
	{FeatureUnprovision, "The unprovision removes what the provision made."},
	{FeatureStart, "The start brings the resource up, as a mount or an activation."},
	{FeatureStop, "The stop brings the resource down."},
	{FeatureRun, "The resource is a task, which runs on a schedule or on demand rather than being up or down."},
	{FeatureSync, "The resource copies or snapshots data, on a schedule or on demand, with the sync actions."},
	{FeatureSnapshot, "The storage of the resource takes point-in-time snapshots, which backups copy from."},
	{FeatureResize, "The resource is grown in place by a resize."},
	{FeatureSignal, "The processes of the resource are sent a signal by the signal action."},
	{FeatureAbort, "The resource stops an orchestrated start before it begins, when starting it would harm, as an address already in use."},
	{FeatureMove, "The resource moves live to another node, by a switch or a takeover with --live."},
	{FeatureEncap, "The resource is a guest om also runs in, which holds encapsulated resources."},
}

type (
	starter interface {
		Start(context.Context) error
	}
	stopper interface {
		Stop(context.Context) error
	}
	runner interface {
		Run(context.Context) error
	}
	updater interface {
		Update(context.Context) error
	}
	signaler interface {
		Signal(context.Context, syscall.Signal) error
	}

	// The sayers are drivers serving several types, which have a feature
	// for some of them only: fs.host makes a filesystem, grows it and
	// has it snapshotted for the types om knows how to, and only checks
	// the mount point of the others, as a nfs.
	provisionsSayer interface {
		Provisions() bool
	}
	resizesSayer interface {
		Resizes() bool
	}
	snapshotsSayer interface {
		TakesSnapshots() bool
	}
)

// FeaturesOf returns the features of the resource r.
func FeaturesOf(r any) []Feature {
	has := map[Feature]bool{}
	_, leader := r.(resource.LeaderProvisioner)
	_, follower := r.(resource.FollowerProvisioner)
	has[FeatureProvision] = leader || follower
	if i, ok := r.(provisionsSayer); ok && !i.Provisions() {
		has[FeatureProvision] = false
	}
	_, leader = r.(resource.LeaderUnprovisioner)
	_, follower = r.(resource.FollowerUnprovisioner)
	has[FeatureUnprovision] = has[FeatureProvision] && (leader || follower)
	_, has[FeatureStart] = r.(starter)
	_, has[FeatureStop] = r.(stopper)
	_, has[FeatureRun] = r.(runner)
	_, has[FeatureSync] = r.(updater)
	_, headSnapshotter := r.(resource.HeadSnapshotter)
	_, snapshotLister := r.(resource.SnapshotLister)
	has[FeatureSnapshot] = headSnapshotter || snapshotLister
	if i, ok := r.(snapshotsSayer); ok && !i.TakesSnapshots() {
		has[FeatureSnapshot] = false
	}
	_, has[FeatureResize] = r.(resource.Resizer)
	if i, ok := r.(resizesSayer); ok && !i.Resizes() {
		has[FeatureResize] = false
	}
	_, has[FeatureSignal] = r.(signaler)
	_, has[FeatureAbort] = r.(resource.Aborter)
	_, has[FeatureMove] = r.(resource.Mover)
	_, has[FeatureEncap] = r.(resource.Encaper)
	l := make([]Feature, 0)
	for _, f := range Features {
		if has[f.Name] {
			l = append(l, f.Name)
		}
	}
	return l
}

// List returns the drivers registered, in a stable order. Only a resource
// driver has features.
func List() []Entry {
	l := make([]Entry, 0)
	for _, id := range driver.List() {
		drv, ok := driver.GetStrict(id)
		if !ok {
			continue
		}
		e := Entry{
			ID:        id,
			Doc:       drv.Doc,
			AliasOf:   drv.AliasOf,
			Selects:   drv.Selects,
			Features:  []Feature{},
			Available: available(id),
		}
		if allocator, ok := drv.Allocator.(func() resource.Driver); ok && id.Group.IsResource() {
			e.Features = FeaturesOf(allocator())
		}
		e.Grant, e.ProvisionGrant = grants(id)
		l = append(l, e)
	}
	return l
}

// grantKind is the kind of the objects the grants of the catalog are the ones
// of: a service is what a user designs, a volume being made from a pool.
const grantKind = naming.KindSvc

// grants returns the grant using the driver id in a service needs, and the
// one only its provision needs beyond it. A keyword needing a grant for some
// values only, as the network of a netns address, needs none here: a value
// without it exists. A virtual driver needs a grant when every driver it
// selects does, and an alias the one of the driver it names.
func grants(id driver.ID) (string, string) {
	drv, ok := driver.GetStrict(id)
	if !ok || !id.Group.IsResource() {
		return "", ""
	}
	if !drv.AliasOf.IsZero() {
		return grants(drv.AliasOf)
	}
	if len(drv.Selects) > 0 {
		var use, provision string
		for i, selected := range drv.Selects {
			g, p := grants(selected)
			if i == 0 {
				use, provision = g, p
				continue
			}
			if g != use {
				use = ""
			}
			if p != provision {
				provision = ""
			}
		}
		return use, provision
	}
	allocator, ok := drv.Allocator.(func() resource.Driver)
	if !ok {
		return "", ""
	}
	m := manifest.Get(allocator())
	if m == nil || !m.Kinds.Has(grantKind) {
		return "", ""
	}
	section := id.Group.String()
	if g := doc.DriverGrant(grantKind, section, id.Name); g != "" {
		return g, ""
	}
	var types []string
	if id.Name != "" {
		types = []string{id.Name}
	}
	var use, provision string
	for _, kw := range m.Keywords() {
		if !kw.Required && !kw.ProvisioningRequired {
			continue
		}
		g, always := doc.KeywordGrant(grantKind, section, kw.Option, types)
		if g == "" || !always {
			continue
		}
		if kw.Required {
			use = g
		} else if provision == "" {
			provision = g
		}
	}
	if use != "" {
		return use, ""
	}
	return "", provision
}

// available says the node has what the driver id needs, as its capabilities
// say: a virtual driver has it when it has one of the drivers it selects,
// and an alias when it has the driver it is another name of.
func available(id driver.ID) bool {
	drv, ok := driver.GetStrict(id)
	if !ok {
		return false
	}
	if !drv.AliasOf.IsZero() {
		return available(drv.AliasOf)
	}
	if len(drv.Selects) == 0 {
		return capabilities.Has(id.Cap())
	}
	for _, selected := range drv.Selects {
		if available(selected) {
			return true
		}
	}
	return false
}

// APIList returns the catalog of the drivers of the node, as the api
// answers it.
func APIList(nodename string) api.DriverList {
	entries := List()
	items := make(api.DriverItems, len(entries))
	for i, e := range entries {
		features := make([]string, len(e.Features))
		for j, f := range e.Features {
			features[j] = string(f)
		}
		selects := make([]string, len(e.Selects))
		for j, id := range e.Selects {
			selects[j] = id.String()
		}
		var aliasOf string
		if !e.AliasOf.IsZero() {
			aliasOf = e.AliasOf.String()
		}
		items[i] = api.DriverItem{
			Kind: "DriverItem",
			Data: api.Driver{
				Name:           e.ID.String(),
				Group:          e.ID.Group.String(),
				Type:           e.ID.Name,
				Doc:            e.Doc,
				Features:       features,
				Selects:        selects,
				AliasOf:        aliasOf,
				Available:      e.Available,
				Grant:          e.Grant,
				ProvisionGrant: e.ProvisionGrant,
			},
			Meta: api.NodeMeta{
				Node: nodename,
			},
		}
	}
	vocabulary := make([]api.DriverFeature, len(Features))
	for i, f := range Features {
		vocabulary[i] = api.DriverFeature{Name: string(f.Name), Doc: f.Doc}
	}
	return api.DriverList{Kind: "DriverList", Items: items, Features: vocabulary}
}
