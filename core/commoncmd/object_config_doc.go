package commoncmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectConfigDoc struct {
		OptsGlobal
		Color   string
		Output  string
		Keyword string
		Driver  string
		Depth   int
	}
)

func NewCmdObjectConfigDoc(kind string) *cobra.Command {
	var options CmdObjectConfigDoc
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "print the keyword documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(kind)
		},
	}
	flags := cmd.Flags()
	FlagObjectSelector(flags, &options.ObjectSelector)
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagKeyword(flags, &options.Keyword)
	FlagDriver(flags, &options.Driver)
	FlagDepth(flags, &options.Depth)
	return cmd
}

func (t *CmdObjectConfigDoc) Run(kind string) error {
	path, err := naming.ParsePath(t.OptsGlobal.ObjectSelector)
	if err != nil {
		path, _ = naming.ParsePath("ns1/" + kind + "/obj1")
	}
	c, err := client.New()
	if err != nil {
		return err
	}

	items, err := doc.FindKeywords(t.Keyword, t.Driver != "", func(section, option *string) (api.KeywordDefinitionItems, error) {
		params := api.GetObjectConfigKeywordsParams{
			Section: section,
			Option:  option,
		}
		if t.Driver != "" {
			params.Driver = &t.Driver
		}
		response, err := c.GetObjectConfigKeywordsWithResponse(
			context.Background(),
			api.InPathNamespace(path.Namespace),
			api.InPathKind(path.Kind),
			api.InPathName(path.Name),
			&params,
		)
		if err != nil {
			return nil, err
		}
		switch {
		case response.JSON200 != nil:
			return response.JSON200.Items, nil
		case response.JSON400 != nil:
			return nil, fmt.Errorf("%s", *response.JSON400)
		case response.JSON401 != nil:
			return nil, fmt.Errorf("%s", *response.JSON401)
		case response.JSON500 != nil:
			return nil, fmt.Errorf("%s", *response.JSON500)
		default:
			return nil, fmt.Errorf("unexpected response: %s", response.Status())
		}
	})
	if err != nil {
		return err
	}

	return output.Renderer{
		HumanRenderer: func() string {
			Doc(os.Stdout, items, path.Kind, t.Driver, t.Keyword, t.Depth)
			return ""
		},
		Output:   t.Output,
		Sort:     t.Sort,
		Color:    t.Color,
		Data:     items,
		Colorize: rawconfig.Colorize,
	}.Print()
}
