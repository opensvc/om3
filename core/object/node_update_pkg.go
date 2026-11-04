package object

import (
	"context"
	"os"

	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pkgupdate"
)

// UpdatePkg upgrades the opensvc packages installed on the node, from the
// repositories its package manager has configured, else from the package
// repository node.repopkg names, https://packages.opensvc.com by default, for
// the branch node.branch, prod by default.
func (t *Node) UpdatePkg(ctx context.Context, dryRun bool) error {
	cfg := t.MergedConfig()
	u := pkgupdate.T{
		Base:   cfg.GetString(key.New("node", "repopkg")),
		Branch: cfg.GetString(key.New("node", "branch")),
		DryRun: dryRun,
		Out:    os.Stdout,
	}
	return u.Run(ctx)
}
