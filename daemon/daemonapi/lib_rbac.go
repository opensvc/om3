package daemonapi

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/keyoprbac"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/util/key"
)

// configRbac validates all keys in a config object against RBAC rules.
// It checks if the user has root grant, and if not, validates each key.
// Returns an error if any key violates RBAC rules.
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
	return configRbacKeys(grants, cfg)
}

// configRbacKeys checks every keyword of a configuration against the policy.
func configRbacKeys(grants rbac.Grants, cfg *xconfig.T) error {
	// Iterate through all sections in the config
	for _, section := range cfg.SectionStrings() {
		// Get all keys in this section
		keys := cfg.Keys(section)
		set := sectionSetter(keys)
		for _, option := range keys {
			k := key.New(section, option)
			// Create a key operation for this key
			v, err := cfg.Eval(k)
			if err != nil {
				return err
			}
			kop := keyop.T{
				Key:   k,
				Op:    keyop.Set,
				Value: configValue(v),
				Index: 0,
			}
			// Validate this key operation against RBAC rules
			if err := keyopRbac(grants, kop, set); err != nil {
				return err
			}
		}
	}
	return nil
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

// keyopRbac refuses a keyword operation the grants of the user are not enough
// for.
//
// What is refused, and why, is the policy in core/keyoprbac, which the keyword
// documentation reads too. Here it is only turned into the error the api
// returns, naming the operation it is about.
func keyopRbac(grants rbac.Grants, op keyop.T, set keyoprbac.Section) error {
	if err := keyoprbac.Denied(grants, op.Key.Section, op.Key.Option, op.Value, set); err != nil {
		return fmt.Errorf("denied: %s: %w", op, err)
	}
	return nil
}

// configValue renders an evaluated keyword value the way the configuration
// spells it, which is what the policy reads.
//
// Evaluating a keyword returns it converted, and a converted list is a
// []string that fmt prints inside brackets. Handing the policy "[_/etc:/etc]"
// where the configuration says "_/etc:/etc" gives it a value that matches
// neither a list of allowed values nor a shape it refuses, so a rule about
// the value would let through exactly what it exists to stop.
func configValue(v any) string {
	switch value := v.(type) {
	case []string:
		return strings.Join(value, " ")
	default:
		return fmt.Sprint(v)
	}
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
