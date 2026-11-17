package object

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pkgupdate"
)

// UpdatePkg upgrades the opensvc packages installed on the node, from the
// repositories its package manager has configured, else from the package
// repository node.repopkg names, https://packages.opensvc.com by default, for
// the branch node.branch, prod by default.
//
// The asset of the node is pushed after the upgrade, so the collector knows
// the new version of the agent without waiting for the asset schedule.
func (t *Node) UpdatePkg(ctx context.Context, dryRun bool) error {
	cfg := t.MergedConfig()
	u := pkgupdate.T{
		Base:   cfg.GetString(key.New("node", "repopkg")),
		Branch: cfg.GetString(key.New("node", "branch")),
		DryRun: dryRun,
		Out:    os.Stdout,
	}
	if err := u.Run(ctx); err != nil {
		return err
	}
	if !dryRun {
		pushAssetAfterUpdatePkg(ctx)
	}
	return nil
}

// pushAssetAfterUpdatePkg pushes the asset of the node with a new om process:
// the running one is the binary before the upgrade, which would report the
// version replaced. A failure is only warned about: the upgrade is done.
func pushAssetAfterUpdatePkg(ctx context.Context) {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	fmt.Fprintln(os.Stdout, "push asset")
	cmd := exec.CommandContext(ctx, exe, "node", "push", "asset")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "push asset after the update of the packages: %s\n", err)
	}
}
