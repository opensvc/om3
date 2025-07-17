package commoncmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/daemon/api"
	"github.com/spf13/cobra"
)

type (
	CmdNodeConfigDoc struct {
		Color   string
		Output  string
		Keyword string
		Driver  string
		Depth   int
	}
)

func NewCmdNodeConfigDoc() *cobra.Command {
	var options CmdNodeConfigDoc
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "print the documentation of the selected keywords",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)
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

	items := make(api.KeywordDefinitionItems, 0)
	index := keywords.ParseIndex(t.Keyword)
	params := api.GetNodeConfigKeywordsParams{}
	if index[0] != "" {
		params.Section = &index[0]
	}
	if index[1] != "" {
		params.Option = &index[1]
	}
	if t.Driver != "" {
		params.Driver = &t.Driver
	}

	response, err := c.GetNodeConfigKeywordsWithResponse(context.Background(), "localhost", &params)
	if err != nil {
		return err
	}
	switch {
	case response.JSON200 != nil:
		items = append(items, response.JSON200.Items...)
	case response.JSON400 != nil:
		return fmt.Errorf("%s", *response.JSON400)
	case response.JSON401 != nil:
		return fmt.Errorf("%s", *response.JSON401)
	case response.JSON500 != nil:
		return fmt.Errorf("%s", *response.JSON500)
	default:
		return fmt.Errorf("unexpected response: %s", response.Status())
	}

	return Doc(os.Stdout, items, "node", t.Depth)
}
