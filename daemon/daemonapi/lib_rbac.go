package daemonapi

import (
	"errors"
	"fmt"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/status"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/keyoprbac"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rootless"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

// configRbac refuses a configuration write the grants do not allow.
//
// What the write changes is what has to be allowed, not what the
// configuration ends up holding. A keyword already there was written by
// whoever was allowed to write it, and asking for that grant again on every
// write would make an object nobody but root can touch out of one that holds a
// single root keyword: the object administrator could not so much as fix a
// comment on a service that mounts a filesystem.
//
// An object being created has nothing to compare against, so every keyword of
// it is a change, and every keyword answers for itself.
func configRbac(ctx echo.Context, p naming.Path, body []byte) error {
	o, err := object.New(p, object.WithConfigData(body), object.WithVolatile(true))
	if err != nil {
		return fmt.Errorf("new object: %s", err)
	}
	configurer := o.(object.Configurer)
	cfg := configurer.Config()
	grants := grantsFromContext(ctx)

	if grants.HasGrant(rbac.GrantRoot) {
		return nil
	}
	from := currentConfig(p)
	if err := configRbacChanges(grants, p.Kind, from, cfg); err != nil {
		return err
	}
	if err := usrRbac(grants, p, from, cfg); err != nil {
		return err
	}
	if err := hostIPRbac(from, cfg); err != nil {
		return err
	}
	if err := removedResourceRbac(p, from, cfg); err != nil {
		return err
	}
	return rootlessRbac(p, from, cfg)
}

// instanceStatusesOf returns the status of the instances of an object, by
// node, replaced by the tests.
var instanceStatusesOf = func(p naming.Path) map[string]*instance.Status {
	return instance.StatusData.GetByPath(p)
}

// removedResourceRbac refuses a write taking away the section of a resource
// still running on a node: an address still configured on an interface, a
// filesystem still mounted.
//
// Taking a section away does not stop its resource, which nothing knows of
// any more once its section is gone: what it holds on the node stays there,
// for root to find and undo, as an address the namespace still has drawn from
// its claim. A user holding no root grant cannot undo it, so they stop the
// resource first, and remove what the object no longer runs.
func removedResourceRbac(p naming.Path, from, to *xconfig.T) error {
	if from == nil {
		return nil
	}
	statuses := instanceStatusesOf(p)
	for _, section := range from.SectionStrings() {
		if slices.Contains(to.SectionStrings(), section) {
			continue
		}
		running := make([]string, 0)
		for nodename, st := range statuses {
			if st == nil {
				continue
			}
			rstat, ok := st.Resources[section]
			if !ok {
				continue
			}
			if !rstat.Status.Is(status.Down, status.StandbyDown, status.NotApplicable) {
				running = append(running, fmt.Sprintf("%s on %s", rstat.Status, nodename))
			}
		}
		if len(running) > 0 {
			slices.Sort(running)
			return fmt.Errorf("%w: delete %s: the resource is not stopped (%s): stop it before removing its section", ErrDenied, section, strings.Join(running, ", "))
		}
	}
	return nil
}

// lookupNetwork returns the network of a name, replaced by the tests.
var lookupNetwork = func(name string) (network.Networker, error) {
	nw, _, err := network.Lookup(name)
	return nw, err
}

// hostIPRbac refuses a write giving an ip.host resource an address of a
// network other than a lan one, or another network than the one root gave an
// address it named.
//
// The keyword policy lets a user holding no root grant have an ip.host draw
// its address from a network, the network saying the address, the interface
// and the netmask. Only a lan network, a range root made for the cluster on a
// segment the nodes share, is one to draw a node address from: the addresses
// of a bridge network are the node's own bridge, and the ones of a
// routed_bridge the subnet of the node. The network type is known here, from
// the cluster configuration, and not where the keywords are.
//
// Only what the write changes is judged: the type or the network of the
// section, on any node the object runs on.
func hostIPRbac(from, to *xconfig.T) error {
	scopes := rbacScopes(from, to)
	defaultType := driver.DefaultDriver[driver.NewGroup("ip")]
	typeOf := func(cfg *xconfig.T, section, nodename string) string {
		if s := evaluatedOrWrittenAs(cfg, key.New(section, "type"), nodename); s != "" {
			return s
		}
		return defaultType
	}
	for _, section := range to.SectionStrings() {
		group, _, _ := strings.Cut(section, "#")
		if group != "ip" {
			continue
		}
		kn := key.New(section, "network")
		for _, nodename := range scopes {
			if typeOf(to, section, nodename) != "host" {
				continue
			}
			name := evaluatedOrWrittenAs(to, kn, nodename)
			if from != nil && len(from.Keys(section)) > 0 &&
				typeOf(from, section, nodename) == "host" &&
				evaluatedOrWrittenAs(from, kn, nodename) == name {
				continue
			}
			if name == "" {
				// The keyword policy refused it, or root named the address.
				continue
			}
			if evaluatedOrWrittenAs(to, key.New(section, "name"), nodename) != "" {
				return fmt.Errorf("%w: %s on %s: the network of an address root named requires the root grant", ErrDenied, section, nodename)
			}
			nw, err := lookupNetwork(name)
			if err != nil {
				return fmt.Errorf("%s: network %s: %w", section, name, err)
			}
			if nw == nil {
				return fmt.Errorf("%w: %s on %s: no network %s to draw the address from", ErrDenied, section, nodename, name)
			}
			if i, ok := nw.(network.ClusterWider); !ok || !i.IsClusterWide() {
				return fmt.Errorf("%w: %s on %s: a host address drawn from network %s, a %s network rather than a lan one, requires the root grant", ErrDenied, section, nodename, name, nw.Type())
			}
		}
	}
	return nil
}

// usrRbac refuses a write of a user giving it more than the writer holds.
//
// A user is its grants: whoever writes the grant keyword of a user, or the
// certificate name it authenticates by, decides what that user may do. So the
// grants a write adds must all be held by the writer, as v2 required, and the
// cn of a user is changed by root alone. Without it, an administrator of the
// system namespace, where the users live, made a user granted root, and was
// root.
//
// The grants the user already holds are not asked for again: an
// administrator can still edit, and take grants away from, a user they could
// not have made.
func usrRbac(grants rbac.Grants, p naming.Path, from, to *xconfig.T) error {
	if p.Kind != naming.KindUsr {
		return nil
	}
	grantKey := key.New("DEFAULT", "grant")
	cnKey := key.New("DEFAULT", "cn")
	var held []string
	// The cn defaults to the name of the user, which is the certificate
	// the user is issued anyway, so a new user may have that one.
	heldCN := p.Name
	if from != nil {
		held = from.GetStrings(grantKey)
		heldCN, _ = from.EvalNoConv(cnKey)
	}
	var added rbac.Grants
	for _, s := range to.GetStrings(grantKey) {
		if !slices.Contains(held, s) {
			added = append(added, rbac.Grant(s))
		}
	}
	if l := grants.Uncovered(added...); len(l) > 0 {
		return fmt.Errorf("%w: %s: granting %s requires holding it", ErrDenied, p, l)
	}
	if cn, _ := to.EvalNoConv(cnKey); cn != heldCN {
		return fmt.Errorf("%w: %s: setting the cn of a user requires the root grant", ErrDenied, p)
	}
	return nil
}

// rootlessRbac refuses a write having a container run as an account its
// namespace does not allow.
//
// The account a rootless container runs as reaches everything else it owns
// on the node, so the namespace configuration lists the ones a namespace may
// use, and a user holding no root grant may not name another. Root is not
// bound by the list.
//
// The account is judged as the keywords evaluate, on every node of the
// object, the way the other keywords are: rootless_user can be a reference,
// or be written for one node alone. Only what the write changes is judged, so
// an object whose account the squatter stopped allowing can still be edited,
// and is said to be running as a disallowed account by its status instead.
func rootlessRbac(p naming.Path, from, to *xconfig.T) error {
	var allowed *rootless.Allowed
	scopes := rbacScopes(from, to)
	for _, section := range to.SectionStrings() {
		group, _, _ := strings.Cut(section, "#")
		if group != "container" && group != "task" {
			continue
		}
		ku := key.New(section, "rootless_user")
		kg := key.New(section, "rootless_group")
		for _, nodename := range scopes {
			u := evaluatedOrWrittenAs(to, ku, nodename)
			g := evaluatedOrWrittenAs(to, kg, nodename)
			if u == "" {
				continue
			}
			if from != nil && len(from.Keys(section)) > 0 &&
				evaluatedOrWrittenAs(from, ku, nodename) == u &&
				evaluatedOrWrittenAs(from, kg, nodename) == g {
				continue
			}
			if allowed == nil {
				a, err := rootless.Load(p.Namespace)
				if err != nil {
					return fmt.Errorf("read the rootless accounts of the %s namespace: %w", p.Namespace, err)
				}
				allowed = &a
			}
			if err := allowed.Check(u, g); err != nil {
				return fmt.Errorf("%w: %s runs as %s on %s: %w", ErrDenied, section, u, nodename, err)
			}
		}
	}
	return nil
}

// currentConfig is the configuration the object holds before the write, and
// nil when it holds none.
//
// A configuration this cannot read is read as none, so the write answers for
// every keyword it lands: a comparison that could not be made is not a
// comparison that found nothing.
func currentConfig(p naming.Path) *xconfig.T {
	if !file.Exists(p.ConfigFile()) {
		return nil
	}
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return nil
	}
	configurer, ok := o.(object.Configurer)
	if !ok {
		return nil
	}
	return configurer.Config()
}

// configRbacChanges checks the keywords a write changes against the policy.
func configRbacChanges(grants rbac.Grants, kind naming.Kind, from, to *xconfig.T) error {
	scopes := rbacScopes(from, to)
	for _, section := range to.SectionStrings() {
		keys := to.Keys(section)
		set := sectionSetter(keys)
		for _, option := range keys {
			k := key.New(section, option)
			nodename, changed := keyChangedOn(from, to, k, scopes)
			if !changed {
				continue
			}
			if followsObjectSize(kind, from, to, k) {
				continue
			}
			// The value judged is the one the keyword takes where it
			// changed, which is not always the one it takes here.
			kop := keyop.T{
				Key:   k,
				Op:    keyop.Set,
				Value: evaluatedOrWrittenAs(to, k, nodename),
				Index: 0,
			}
			if err := keyopRbacOn(grants, kind, kop, set, nodename); err != nil {
				return err
			}
		}
		if err := sectionDriverRbac(grants, kind, from, to, section, set, scopes); err != nil {
			return err
		}
	}
	if from == nil {
		return nil
	}
	// A keyword the write takes away is a change like the ones it makes. A
	// user who may not set a keyword may not unset it either, and a section
	// deleted is every keyword of it unset.
	for _, section := range from.SectionStrings() {
		keys := from.Keys(section)
		set := sectionSetter(keys)
		for _, option := range keys {
			k := key.New(section, option)
			if to.HasKey(k) {
				continue
			}
			if err := keyUnsetRbac(grants, kind, k, evaluatedOrWritten(from, k), set); err != nil {
				return err
			}
		}
	}
	return nil
}

// sectionDriverRbac checks the driver a section runs when it names none.
//
// A rule about a driver type is asked about a keyword, and a section writing
// no type gives it no keyword to ask about. It runs the driver its group falls
// back to all the same: a task writing no type runs its command on the node
// exactly as "type = host" does, and that is refused where the fallback is not
// written only because nothing looked.
//
// So the fallback is checked as though it had been written. It is checked when
// the section is new to the object, and when the write is what stopped the
// section naming a type, which are the two ways a section comes to run a
// driver it does not name.
//
// It is asked node by node, because a type is a keyword like any other and can
// be written for one node alone: a section naming a type on one node names
// nothing on the others, and runs the fallback there. Reading "a type is
// written" as "a type is written everywhere" turned the check into a way
// around itself.
func sectionDriverRbac(grants rbac.Grants, kind naming.Kind, from, to *xconfig.T, section string, set keyoprbac.Section, scopes []string) error {
	group, _, _ := strings.Cut(section, "#")
	name := driver.DefaultDriver[driver.NewGroup(group)]
	if name == "" {
		// A group with no fallback runs nothing it was not told to run.
		return nil
	}
	k := key.New(section, "type")
	for _, nodename := range scopes {
		if to.GetStringAs(k, nodename) != "" {
			// The section names what it runs there, and that keyword was
			// checked as the keyword it is.
			continue
		}
		if from != nil && len(from.Keys(section)) > 0 && from.GetStringAs(k, nodename) == "" {
			// The section ran this driver there before the write too.
			continue
		}
		if err := keyoprbac.Denied(grants, kind, section, "type", name, set); err != nil {
			return fmt.Errorf("%w: %s runs the %s driver on %s, which it does not name: %w", ErrDenied, section, name, nodename, err)
		}
	}
	return nil
}

// followsObjectSize says a keyword changed only because the size of the
// volume it points at changed.
//
// A pool writes the size of the resource it serves as a reference to
// DEFAULT.size, so that growing the volume grows the storage and the claim
// the cluster rations it by, together and in step. Reading that as a write of
// a volume resource would leave the administrator of the namespace unable to
// grow the volume at all, which is the one thing a claim is for.
//
// It is the narrowest reading of that arrangement: the size option, the whole
// value a reference to the size of the object, and the same before the write
// and after it. A keyword pointed at another one is not opened by it
// otherwise, so a root-only "pre_start = {env.cmd}" still answers for a write
// of the env keyword it reads.
func followsObjectSize(kind naming.Kind, from, to *xconfig.T, k key.T) bool {
	const ref = "{DEFAULT.size}"
	if kind != naming.KindVol || k.Option != "size" || k.Section == "DEFAULT" {
		return false
	}
	return from != nil && from.Get(k) == ref && to.Get(k) == ref
}

// keyChangedOn says whether a write changes what a keyword means, and on
// which node it changed it.
//
// The written value is not the whole of it. A keyword can hold a reference,
// and moving what it refers to changes the keyword without touching it: a
// root-only "pre_start = {env.cmd}" is rewritten by any write of env.cmd,
// however many references deep it sits. So the values are compared as they
// evaluate, which is what resolves those references.
//
// They are compared on every node the object runs on, before and after,
// because a keyword can be written once per node: a reference that resolves
// the same here can resolve to something else on a peer. A node the write
// adds makes every keyword new there, which is the comparison finding nothing
// to compare against.
func keyChangedOn(from, to *xconfig.T, k key.T, scopes []string) (string, bool) {
	localhost := hostname.Hostname()
	if from == nil || !from.HasKey(k) {
		return localhost, true
	}
	if from.Get(k) != to.Get(k) {
		return localhost, true
	}
	// This node is asked first, so a keyword that changed everywhere is
	// answered for plainly rather than named after a peer.
	for _, nodename := range append([]string{localhost}, scopes...) {
		a, errA := from.EvalAs(k, nodename)
		b, errB := to.EvalAs(k, nodename)
		if errA != nil || errB != nil {
			// A value that cannot be read cannot be said to be unchanged.
			return nodename, true
		}
		if xconfig.EvaluatedString(a) != xconfig.EvaluatedString(b) {
			return nodename, true
		}
	}
	return "", false
}

// rbacScopes is the nodes the keywords are compared on: the ones the object
// runs on before the write and after it, and this one, which answers for a
// configuration that names no node.
func rbacScopes(cfgs ...*xconfig.T) []string {
	k := key.New("DEFAULT", "nodes")
	set := map[string]bool{hostname.Hostname(): true}
	for _, cfg := range cfgs {
		if cfg == nil {
			continue
		}
		for _, nodename := range cfg.GetStrings(k) {
			set[nodename] = true
		}
	}
	l := make([]string, 0, len(set))
	for nodename := range set {
		l = append(l, nodename)
	}
	sort.Strings(l)
	return l
}

// evaluatedOrWritten is a keyword value as it evaluates, or as it is written
// when it no longer evaluates. A rule reading the value is then given the
// reference itself, which no rule opens, rather than nothing at all.
func evaluatedOrWritten(cfg *xconfig.T, k key.T) string {
	return evaluatedOrWrittenAs(cfg, k, hostname.Hostname())
}

// evaluatedOrWrittenAs is evaluatedOrWritten in the scope the keyword is
// judged in.
//
// What a keyword evaluates to is not always readable: a node selector needs
// the nodes the cluster knows, and a configuration written where they cannot
// be read has keywords nothing can resolve. A write is not refused for it.
// The value is read as it is written instead, which is what the policy is
// given, and a rule looking for a value it names does not find it in a
// reference.
func evaluatedOrWrittenAs(cfg *xconfig.T, k key.T, nodename string) string {
	v, err := cfg.EvalAs(k, nodename)
	if err != nil {
		return cfg.Get(k)
	}
	return xconfig.EvaluatedString(v)
}

// assertGuest asserts that the authenticated user has is either granted the "guest", "operator" or "admin" role on the namespace or is granted the "root" role.
func assertGuest(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx,
		rbac.NewGrant(rbac.RoleGuest, namespace),
		rbac.NewGrant(rbac.RoleOperator, namespace),
		rbac.NewGrant(rbac.RoleAdmin, namespace),
		rbac.NewGrant(rbac.RoleGuest, ""),
		rbac.NewGrant(rbac.RoleOperator, ""),
		rbac.NewGrant(rbac.RoleAdmin, ""),
		rbac.GrantJoin,
		rbac.GrantRoot,
	)
}

// assertOperator asserts that the authenticated user has is either granted the "operator" or "admin" role on the namespace or is granted the "root" role.
func assertOperator(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx,
		rbac.NewGrant(rbac.RoleOperator, namespace),
		rbac.NewGrant(rbac.RoleAdmin, namespace),
		rbac.NewGrant(rbac.RoleOperator, ""),
		rbac.NewGrant(rbac.RoleAdmin, ""),
		rbac.GrantRoot,
	)
}

// assertAdmin asserts that the authenticated user has is either granted the "admin" role on the namespace or is granted the "root" role.
func assertAdmin(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx,
		rbac.NewGrant(rbac.RoleAdmin, namespace),
		rbac.NewGrant(rbac.RoleAdmin, ""),
		rbac.GrantRoot,
	)
}

// assertNamespaceConfigWriter asserts the authenticated user may write the
// configuration of a namespace, which is also what creating one is.
//
// A namespace configuration says what the objects of the namespace may take of
// the resources the cluster shares. That is not the namespace administrator's
// to write, whatever admin of it they hold: a limit is what weighs one
// namespace against the others, so it is given from outside, by the squatter
// rationing the cluster between them.
func assertNamespaceConfigWriter(ctx echo.Context) (bool, error) {
	return assertGrant(ctx, rbac.GrantSquatter, rbac.GrantRoot)
}

// assertRoot asserts that the authenticated user has is granted the "root" role.
func assertRoot(ctx echo.Context) (bool, error) {
	return assertGrant(ctx, rbac.GrantRoot)
}

func assertStrategy(ctx echo.Context, expected string) (bool, error) {
	if strategy := strategyFromContext(ctx); strategy != expected {
		return false, JSONForbiddenStrategy(ctx, strategy, expected)
	}
	return true, nil
}

func assertGrant(ctx echo.Context, grants ...rbac.Grant) (bool, error) {
	if !grantsFromContext(ctx).HasGrant(grants...) {
		return false, JSONForbiddenMissingGrant(ctx, grants...)
	}
	return true, nil
}

func assertRole(ctx echo.Context, roles ...rbac.Role) (bool, error) {
	if !grantsFromContext(ctx).HasRole(roles...) {
		return false, JSONForbiddenMissingRole(ctx, roles...)
	}
	return true, nil
}

// ErrDenied heads every refusal the keyword policy makes, so a caller deep
// enough to have lost sight of why an update failed can still answer with the
// forbidden it is rather than an internal error.
var ErrDenied = errors.New("denied")

// keyopRbac refuses a keyword operation the grants of the user are not enough
// for.
//
// What is refused, and why, is the policy in core/keyoprbac, which the keyword
// documentation reads too. Here it is only turned into the error the api
// returns, naming the operation it is about.
// keyopRbacOn refuses a write of a keyword, naming the node the keyword takes
// that value on when it is not this one: a keyword written once per node is
// refused for what it does where it changed, which the message has to say or
// it names a value the configuration does not hold here.
func keyopRbacOn(grants rbac.Grants, kind naming.Kind, op keyop.T, set keyoprbac.Section, nodename string) error {
	err := keyoprbac.Denied(grants, kind, op.Key.Section, op.Key.Option, op.Value, set)
	switch {
	case err == nil:
		return nil
	case nodename == "" || nodename == hostname.Hostname():
		return fmt.Errorf("%w: %s: %w", ErrDenied, op, err)
	default:
		return fmt.Errorf("%w: %s on %s: %w", ErrDenied, op, nodename, err)
	}
}

// keyUnsetRbac refuses taking a keyword away, which is judged on the value it
// is being taken away from: a user who may not set a keyword to a value may
// not unset it from that value either.
func keyUnsetRbac(grants rbac.Grants, kind naming.Kind, k key.T, value string, set keyoprbac.Section) error {
	if err := keyoprbac.DeniedUnset(grants, kind, k.Section, k.Option, value, set); err != nil {
		return fmt.Errorf("%w: unset %s: %w", ErrDenied, k, err)
	}
	return nil
}

// sectionSetter answers whether an option is set in a section, from the keys
// the section holds.
//
// A key scoped to a node is the same keyword as an unscoped one, and a config
// naming an address only on a peer node names it here too: this reads the keys
// as written rather than as evaluated for this node, so a scope is not a way
// around a rule about the setup.
func sectionSetter(keys []string) keyoprbac.Section {
	set := make(map[string]bool, len(keys))
	for _, option := range keys {
		option, _, _ = strings.Cut(option, "@")
		set[option] = true
	}
	return func(option string) bool {
		return set[option]
	}
}

func hasRoleGuestOn(grants rbac.Grants, namespace string) bool {
	return grants.AssertRoleOn(namespace, rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin)
}

// hasRoleOperatorOn checks if the Grants contain either `RoleOperator` or `RoleAdmin` for the specified `namespace`.
func hasRoleOperatorOn(grants rbac.Grants, namespace string) bool {
	return grants.AssertRoleOn(namespace, rbac.RoleOperator, rbac.RoleAdmin)
}

// hasRoleAdminOn determines if the given grants contain the `RoleAdmin` for the specified `namespace`.
func hasRoleAdminOn(grants rbac.Grants, namespace string) bool {
	return grants.AssertRoleOn(namespace, rbac.RoleAdmin)
}

// assertUsrKeyWrite refuses a write of a key of a user whose grants the
// writer does not all hold.
//
// The keys of a user are its credentials, its password and its certificate,
// and whoever sets them authenticates as that user. So writing them is taking
// the grants of the user, and needs holding them already: an administrator of
// the system namespace can manage the users up to their own grants, and not a
// user granted root. v2 let an administrator reset the password of any user,
// which made the system namespace administrator root.
//
// It reads the user configuration of this node, so it is asked on the node
// that writes the key.
func assertUsrKeyWrite(ctx echo.Context, p naming.Path) (bool, error) {
	if p.Kind != naming.KindUsr {
		return true, nil
	}
	grants := grantsFromContext(ctx)
	if grants.HasGrant(rbac.GrantRoot) {
		return true, nil
	}
	cfg := currentConfig(p)
	if cfg == nil {
		return false, JSONProblemf(ctx, http.StatusNotFound, "Not found", "%s: no configuration on this node", p)
	}
	var held rbac.Grants
	for _, s := range cfg.GetStrings(key.New("DEFAULT", "grant")) {
		held = append(held, rbac.Grant(s))
	}
	if l := grants.Uncovered(held...); len(l) > 0 {
		return false, JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "%s: writing the keys of a user requires holding its grants, missing %s", p, l)
	}
	return true, nil
}
