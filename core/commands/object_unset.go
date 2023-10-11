package commands

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectaction"
	"github.com/opensvc/om3/core/objectlogger"
	"github.com/opensvc/om3/util/key"
)

type (
	CmdObjectUnset struct {
		OptsGlobal
		OptsLock
		Keywords []string
		Sections []string
	}
)

func (t *CmdObjectUnset) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.LocalFirst(),
		objectaction.WithLocal(t.Local),
		objectaction.WithColor(t.Color),
		objectaction.WithOutput(t.Output),
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteAction("unset"),
		objectaction.WithRemoteOptions(map[string]interface{}{
			"kw":       t.Keywords,
			"sections": t.Sections,
		}),
		objectaction.WithLocalRun(func(ctx context.Context, p naming.Path) (interface{}, error) {
			// TODO: one commit on Unset, one commit on DeleteSection. Change to single commit ?
			logger := objectlogger.New(p,
				objectlogger.WithColor(t.Color != "no"),
				objectlogger.WithConsoleLog(t.Log != ""),
				objectlogger.WithLogFile(true),
			)
			o, err := object.NewConfigurer(p, object.WithLogger(logger))
			if err != nil {
				return nil, err
			}
			ctx = actioncontext.WithLockDisabled(ctx, t.Disable)
			ctx = actioncontext.WithLockTimeout(ctx, t.Timeout)
			kws := key.ParseStrings(t.Keywords)
			if len(kws) > 0 {
				log.Debug().Msgf("unsetting %s keywords: %s", p, kws)
				if err = o.Unset(ctx, kws...); err != nil {
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
				log.Debug().Msgf("deleting %s sections: %s", p, sections)
				if err = o.DeleteSection(ctx, sections...); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}),
	).Do()
}
