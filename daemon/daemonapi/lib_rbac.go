package daemonapi

import (
	"fmt"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
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

	// Iterate through all sections in the config
	for _, section := range cfg.SectionStrings() {
		// Get all keys in this section
		keys := cfg.Keys(section)
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
				Value: fmt.Sprint(v),
				Index: 0,
			}
			// Validate this key operation against RBAC rules
			if err := keyopRbac(grants, kop); err != nil {
				return err
			}
		}
	}
	return nil
}

// assertGuest asserts that the authenticated user has is either granted the "guest", "operator" or "admin" role on the namespace or is granted the "root" role.
func assertGuest(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx, rbac.NewGrant(rbac.RoleGuest, namespace), rbac.NewGrant(rbac.RoleOperator, namespace), rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantJoin, rbac.GrantRoot)
}

// assertOperator asserts that the authenticated user has is either granted the "operator" or "admin" role on the namespace or is granted the "root" role.
func assertOperator(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx, rbac.NewGrant(rbac.RoleOperator, namespace), rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot)
}

// assertAdmin asserts that the authenticated user has is either granted the "admin" role on the namespace or is granted the "root" role.
func assertAdmin(ctx echo.Context, namespace string) (bool, error) {
	return assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot)
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

func lineHasLocalSource(words []string) bool {
	var word string
	for {
		word, words = datarecv.Pop(words)
		if word == "" {
			break
		}
		switch word {
		case "source":
			word, words = datarecv.Pop(words)
			if !strings.HasPrefix(word, "http://") && !strings.HasPrefix(word, "https://") {
				return true
			}
		}
	}
	return false
}

func textHasLocalSource(s string) bool {
	text, _ := shlex.Split(s, true)
	for _, line := range datarecv.Split(text) {
		if lineHasLocalSource(line) {
			return true
		}
	}
	return false
}

func isActionTrigger(s string) bool {
	switch s {
	case "blocking_post_provision":
	case "blocking_post_run":
	case "blocking_post_start":
	case "blocking_post_stop":
	case "blocking_post_unprovision":

	case "blocking_pre_provision":
	case "blocking_pre_run":
	case "blocking_pre_start":
	case "blocking_pre_stop":
	case "blocking_pre_unprovision":

	case "post_provision":
	case "post_run":
	case "post_start":
	case "post_stop":
	case "post_unprovision":

	case "pre_provision":
	case "pre_run":
	case "pre_start":
	case "pre_stop":
	case "pre_unprovision":

	default:
		return false
	}
	return true
}

func keyopRbac(grants rbac.Grants, op keyop.T) error {
	option := op.Key.Option

	// Strip the scoping suffix
	before, _, found := strings.Cut(option, "@")
	if found {
		option = before
	}

	if isActionTrigger(option) {
		return fmt.Errorf("denied: %s: triggers requires the root grant", op)
	}

	drvGroup := strings.Split(op.Key.Section, "#")[0]
	switch drvGroup {
	case "task":
		switch option {
		case "type":
			switch op.Value {
			case "oci", "docker", "podman":
			default:
				return fmt.Errorf("denied: %s: type requires the root grant", op)
			}
		case "run_args":
			return fmt.Errorf("denied: %s: requires the root grant", op)
		}
	case "container":
		switch option {
		case "type":
			switch op.Value {
			case "oci", "docker", "podman":
			default:
				return fmt.Errorf("denied: %s: type requires the root grant", op)
			}
		case "volume_mounts":
			for _, e := range strings.Fields(op.Value) {
				if strings.HasPrefix(e, "_") || strings.Contains(e, "/../") || strings.HasPrefix(e, "../") || strings.HasSuffix(e, "../") {
					return fmt.Errorf("denied: %s: host path mounts in container require the root grant", op)
				}
			}
		case "run_args":
			return fmt.Errorf("denied: %s: requires the root grant", op)
		}
	case "volume":
		switch option {
		case "install":
			if textHasLocalSource(op.Value) {
				return fmt.Errorf("denied: %s: server-local source uri requires the root grant", op)
			}
		}
	case "fs":
		switch option {
		case "type":
			switch op.Value {
			case "flag":
			default:
				return fmt.Errorf("denied: %s: type requires the root grant", op)
			}
		}
	case "ip":
		switch option {
		case "type":
			switch op.Value {
			case "cni":
			default:
				return fmt.Errorf("denied: %s: type requires the root grant", op)
			}
		}
	case "DEFAULT":
		switch option {
		case "priority":
			// Priorities have cross-namespaces consequences, so require GrantRoot or a dedicated GrantPrioritizer
			if !grants.HasGrant(rbac.GrantPrioritizer) {
				return fmt.Errorf("denied: %s: requires the prioritizer grant", op)
			}
		case "monitor_action":
			switch op.Value {
			case "switch", "freezestop", "none":
			default:
				return fmt.Errorf("denied: %s: requires the root grant", op)
			}
		case "pre_monitor_action":
			return fmt.Errorf("denied: %s: requires the root grant", op)

		}
	case "env":
		// allowed
	default:
		return fmt.Errorf("denied: %s: this driver group requires the root grant", op)
	}
	return nil
}

// hasRoleGuestOn checks if the given `grants` contains the roles `guest`, `operator` or `admin` for the specified `namespace`.
func hasRoleGuestOn(grants rbac.Grants, namespace string) bool {
	return grants.HasRoleOn(namespace, rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin)
}

// hasRoleOperatorOn checks if the Grants contain either `RoleOperator` or `RoleAdmin` for the specified `namespace`.
func hasRoleOperatorOn(grants rbac.Grants, namespace string) bool {
	return grants.HasRoleOn(namespace, rbac.RoleOperator, rbac.RoleAdmin)
}

// hasRoleAdminOn determines if the given grants contain the `RoleAdmin` for the specified `namespace`.
func hasRoleAdminOn(grants rbac.Grants, namespace string) bool {
	return grants.HasRoleOn(namespace, rbac.RoleAdmin)
}
