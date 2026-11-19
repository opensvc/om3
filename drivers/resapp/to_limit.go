package resapp

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/capability"
	"github.com/opensvc/om3/v3/util/capexec"
	"github.com/opensvc/om3/v3/util/converters"
)

// toCaps returns the cappings and limits om exec applies to the action
// command, including the identity it demotes to. The credential is
// decided by the policy in credential(), not by the user and group
// keywords alone.
//
// A privilege restriction whose value is not one is an error, failing the
// command: converted with the other keywords, it would be left unset, and
// the command run unrestricted.
func (t *T) toCaps(cred execCredential) (capexec.T, error) {
	xo := capexec.T{}
	//xo.LoadPG(*t.GetPG())
	pg := *t.GetPG()
	if pg.ID != "" {
		xo.PGID = &pg.ID
	}
	if cred.User != "" {
		xo.User = &cred.User
	}
	if cred.Group != "" {
		xo.Group = &cred.Group
	}
	if t.Umask != nil {
		umask := fmt.Sprintf("%03o", uint32(*t.Umask))
		xo.Umask = &umask
	}
	if t.NoNewPrivileges != "" {
		v, err := converters.Bool.Convert(t.NoNewPrivileges)
		if err != nil {
			return xo, fmt.Errorf("no_new_privileges: %w", err)
		}
		if v.(bool) {
			b := true
			xo.NoNewPrivileges = &b
		}
	}
	if t.CapabilityBoundingSet != "" {
		bs, err := capability.ParseBoundingSet(t.CapabilityBoundingSet)
		if err != nil {
			return xo, fmt.Errorf("capability_bounding_set: %w", err)
		}
		s := bs.String()
		xo.CapabilityBoundingSet = &s
	}
	xo.LoadLimit(t.Limit)
	return xo, nil
}

// validateBool refuses a value that is no boolean, for the keywords whose
// value is converted when the command is built rather than when the
// resource is configured.
func validateBool(s string) error {
	_, err := converters.Bool.Convert(s)
	return err
}

// validateCapabilityBoundingSet refuses a capability bounding set naming a
// capability that does not exist.
func validateCapabilityBoundingSet(s string) error {
	_, err := capability.ParseBoundingSet(s)
	return err
}
