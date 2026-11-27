package object

import (
	"slices"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// pullSource reads a resource section for the image it pulls, as this node
// evaluates it, with the keywords of the driver the node runs it with: what
// the driver is configured with at a start.
type pullSource struct {
	cfg     *xconfig.T
	path    naming.Path
	section string
	kws     keywords.Store
	typ     string
}

func (t pullSource) Get(option string) (string, bool) {
	k := key.New(t.section, option)
	kw := t.kws.Lookup(k, t.path.Kind, t.typ)
	if kw == nil {
		return "", false
	}
	v, err := t.cfg.EvalKeywordAs(k, kw, "")
	if err != nil {
		return "", false
	}
	return xconfig.EvaluatedString(v), true
}

func (t pullSource) Path() naming.Path { return t.path }

func (t pullSource) RID() string { return t.section }

// Pulls returns the images this node prefetches for the object, by the rid
// of the resource running a container from each, with the rids of the
// resources whose image keyword evaluates to no image, and are left out.
//
// The image of a resource is the one its start would run on this node: its
// keywords are evaluated as the driver evaluates them, so the configurer is
// the one of an action, not one evaluating from the configuration alone.
//
// A resource is prefetched when its image_prefetch keyword says so, and as
// the node.image_prefetch keyword of the node says when it says nothing:
// that one is read from the configuration of the node merged over the one of
// the cluster, where the cluster sets the default of its nodes. A resource
// disabled is never started, and an encap one runs in its guest: neither
// pulls an image on this node. The tasks run a container when asked or
// scheduled, not at a failover, and say no Pull.
func Pulls(p naming.Path, oc Configurer, n *Node) (map[string]resource.Pull, []string) {
	cfg := oc.Config()
	pulls := make(map[string]resource.Pull)
	unevaluated := make([]string, 0)
	if cfg.GetBool(key.New("DEFAULT", "disable")) {
		return pulls, unevaluated
	}
	var (
		engine      string
		nodeDefault *bool
	)
	for _, section := range cfg.SectionStrings() {
		rid, err := resourceid.Parse(section)
		if err != nil || rid.DriverGroup() != driver.GroupContainer {
			continue
		}
		typ := cfg.GetString(key.New(section, "type"))
		if typ == "" {
			typ = driver.DefaultDriver[driver.GroupContainer]
		}
		if typ == "oci" {
			// The engine of the node, as the resources are configured
			// with: see actor.ConfigureResources.
			if engine == "" {
				engine = n.OCIEngine()
			}
			typ = engine
		}
		drv := sectionPullDescriber(typ)
		if drv == nil {
			continue
		}
		if cfg.GetBool(key.New(section, "disable")) || cfg.GetBool(key.New(section, "encap")) {
			continue
		}
		src := pullSource{
			cfg:     cfg,
			path:    p,
			section: section,
			kws:     keywords.Store(manifest.Get(drv).Keywords()),
			typ:     typ,
		}
		switch enabled, _ := src.Get("image_prefetch"); enabled {
		case "true":
		case "false":
			continue
		default:
			if nodeDefault == nil {
				b := n.MergedConfig().GetBool(key.New("node", "image_prefetch"))
				nodeDefault = &b
			}
			if !*nodeDefault {
				continue
			}
		}
		pull, ok := drv.(resource.PullDescriber).PullOf(src)
		if !ok {
			unevaluated = append(unevaluated, section)
			continue
		}
		pulls[section] = pull
	}
	slices.Sort(unevaluated)
	return pulls, unevaluated
}

// sectionPullDescriber returns the container driver of the type, allocated
// and not configured, when its resources pull an image, and nil otherwise.
func sectionPullDescriber(typ string) resource.Driver {
	for _, i := range driver.All.WithID(driver.GroupContainer, typ) {
		allocator, ok := i.Allocator.(func() resource.Driver)
		if !ok {
			continue
		}
		drv := allocator()
		if _, ok := drv.(resource.PullDescriber); ok {
			return drv
		}
	}
	return nil
}
