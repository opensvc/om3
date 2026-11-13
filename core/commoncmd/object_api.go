package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/opensvc/om3/v3/core/actionrouter"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xerrors"
)

// ObjectAPIFunc does the work of a command for one object, through the api.
type ObjectAPIFunc func(context.Context, *client.T, naming.Path) (any, error)

// ObjectsAPIDo runs fn for each object the selector expands to, through the
// api: for the commands whose work no instance does, as an attachment held
// by the collector, which the daemon receiving the request asks for the
// object. The results render as the local command renders them, each under
// its object when there are several.
func ObjectsAPIDo(objectSelector string, opts OptsGlobal, fn ObjectAPIFunc) error {
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	paths, err := objectselector.New(objectSelector, objectselector.WithClient(c)).MustExpand()
	if errors.Is(err, xerrors.ObjectNotFound) && opts.IgnoreNotFound {
		return nil
	} else if err != nil {
		return err
	}
	var errs error
	results := make([]actionrouter.Result, 0, len(paths))
	ctx := context.Background()
	for _, p := range paths {
		data, err := fn(ctx, c, p)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			errs = errors.New("api action error")
			continue
		}
		results = append(results, actionrouter.Result{
			Path:          p,
			Data:          data,
			HumanRenderer: func() string { return actionrouter.DefaultHumanRenderer(data) },
		})
	}
	return errors.Join(errs, output.Renderer{
		Output:   opts.Output,
		Sort:     opts.Sort,
		Color:    opts.Color,
		Data:     results,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return humanObjectResults(results)
		},
	}.Print())
}

func humanObjectResults(results []actionrouter.Result) string {
	sort.SliceStable(results, func(i, j int) bool { return results[i].Path.String() < results[j].Path.String() })
	s := ""
	for _, r := range results {
		body := r.HumanRenderer()
		if len(results) > 1 && body != "" {
			s += rawconfig.Colorize.Bold(r.Path.String()) + "\n"
		}
		s += body
	}
	return s
}
