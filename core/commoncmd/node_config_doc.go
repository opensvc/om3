package commoncmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdNodeConfigDoc struct {
		Color   string
		Output  string
		Sort    string
		Keyword string
		Driver  string
		Depth   int
	}
)

func NewCmdNodeConfigDoc() *cobra.Command {
	var options CmdNodeConfigDoc
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "print the keyword documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
	FlagSort(flags, &options.Sort)
	FlagKeyword(flags, &options.Keyword)
	FlagDriver(flags, &options.Driver)
	FlagDepth(flags, &options.Depth)
	cmd.MarkFlagsMutuallyExclusive("driver", "kw")
	return cmd
}

func (t *CmdNodeConfigDoc) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	items, err := doc.FindKeywords(t.Keyword, t.Driver != "", func(section, option *string) (api.KeywordDefinitionItems, error) {
		params := api.GetNodeConfigKeywordsParams{
			Section: section,
			Option:  option,
		}
		if t.Driver != "" {
			params.Driver = &t.Driver
		}
		response, err := c.GetNodeConfigKeywordsWithResponse(context.Background(), "localhost", &params)
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
			Doc(os.Stdout, items, "node", t.Driver, t.Keyword, t.Depth)
			return ""
		},
		Output:   t.Output,
		Sort:     t.Sort,
		Color:    t.Color,
		Data:     items,
		Colorize: rawconfig.Colorize,
	}.Print()
}
