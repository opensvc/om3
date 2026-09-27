package daemonapi

import (
	"errors"
	"fmt"
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
	return rootlessRbac(p, from, cfg)
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
	if err := keyoprbac.Denied(grants, kind, k.Section, k.Option, value, set); err != nil {
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
