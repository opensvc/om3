package object

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/util/key"
)

// TriggerCommand returns the command of the trigger hook of the resource
// rid, as blocking_post_provision, evaluated now, as the trigger runs.
//
// The command of a resource is evaluated when the action reaches the
// resource, before it acts: a post trigger naming what the action makes, as
// the mount point of the volume the resource provisions, would read it
// empty. Evaluated at the time it runs, it reads what the action made.
//
// A reference evaluating only at run time that is not available, as the
// mount point of a volume not provisioned or the devices of a disk not
// started, fails the evaluation, rather than run the command with a path
// left empty: "rm -rf {volume#1.mnt}/cache" would remove /cache.
func (t *actor) TriggerCommand(rid, hook string) (string, error) {
	k := key.New(rid, hook)
	v, err := t.config.EvalStrictAs(k, "", judgeRuntimeReference)
	if err != nil {
		return "", fmt.Errorf("%s: %w", k, err)
	}
	s, _ := v.(string)
	return s, nil
}

// judgeRuntimeReference refuses a reference evaluating only at run time that
// failed or evaluated empty: nothing a command can act on.
func judgeRuntimeReference(ref, val string, err error) error {
	if !isRuntimeReference(ref) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("{%s}: %w", ref, err)
	}
	if strings.TrimSpace(val) == "" {
		return fmt.Errorf("{%s} evaluates empty", ref)
	}
	return nil
}
