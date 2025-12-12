package commoncmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/spf13/cobra"
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

	items := make(api.KeywordDefinitionItems, 0)
	index := keywords.ParseIndex(t.Keyword)
	params := api.GetObjectConfigKeywordsParams{}

	if index[0] != "" {
		params.Section = &index[0]
	}
	if index[1] != "" {
		params.Option = &index[1]
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
	output.Renderer{
		HumanRenderer: func() string {
			Doc(os.Stdout, items, path.Kind, t.Driver, t.Keyword, t.Depth)
			return ""
		},
		Output:   t.Output,
		Color:    t.Color,
		Data:     items,
		Colorize: rawconfig.Colorize,
	}.Print()
	return nil
}
