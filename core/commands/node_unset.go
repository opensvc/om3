package commands

import (
	"context"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/nodeaction"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdNodeUnset struct {
		OptsGlobal
		OptsLock
		Keywords []string
		Sections []string
	}
)

func (t *CmdNodeUnset) Run() error {
	return nodeaction.New(
		nodeaction.LocalFirst(),
		nodeaction.WithLocal(t.Local),
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithColor(t.Color),
		nodeaction.WithServer(t.Server),
		nodeaction.WithRemoteAction("unset"),
		nodeaction.WithRemoteOptions(map[string]interface{}{
			"kw":       t.Keywords,
			"sections": t.Sections,
		}),
		nodeaction.WithLocalRun(func() (interface{}, error) {
			// TODO: one commit on Unset, one commit on DeleteSection. Change to single commit ?
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			ctx := context.Background()
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			kws := key.ParseStrings(t.Keywords)
			if len(kws) > 0 {
				n.Log().Debugf("unsetting node keywords: %s", kws)
				if err = n.Unset(ctx, kws...); err != nil {
					return nil, err
				}
			}
			sections := make([]string, 0)
			for _, r := range t.Sections {
				if r != "DEFAULT" {
					sections = append(sections, r)
				}
			}
			if len(sections) > 0 {
				n.Log().Debugf("deleting node sections: %s", sections)
				if err = n.DeleteSection(sections...); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}),
	).Do()
}
