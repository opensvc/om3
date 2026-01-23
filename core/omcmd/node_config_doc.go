package omcmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
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
		Short: "print the keyword documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	commoncmd.FlagColor(flags, &options.Color)
	commoncmd.FlagOutput(flags, &options.Output)
	commoncmd.FlagKeyword(flags, &options.Keyword)
	commoncmd.FlagDriver(flags, &options.Driver)
	commoncmd.FlagDepth(flags, &options.Depth)
	return cmd
}

func (t *CmdNodeConfigDoc) Run() error {
	var path naming.Path
	var driver, section, option *string
	if t.Driver != "" {
		driver = &t.Driver
	}
	if t.Keyword != "" {
		index := keywords.ParseIndex(t.Keyword)
		section = &index[0]
		option = &index[1]
	}
	store := object.NodeKeywordStore
	store, err := doc.FilterKeywordStore(store, driver, section, option, path, func() (doc.ConfigProvider, error) {
		var (
			i   any
			err error
		)
		i, err = object.NewNode(object.WithVolatile(true))
		if err != nil {
			return nil, err
		}
		return i.(doc.ConfigProvider), nil
	})
	if err != nil {
		return err
	}
	items := doc.ConvertKeywordStore(store)
	output.Renderer{
		HumanRenderer: func() string {
			commoncmd.Doc(os.Stdout, items, path.Kind, t.Driver, t.Keyword, t.Depth)
			return ""
		},
		Output:   t.Output,
		Color:    t.Color,
		Data:     items,
		Colorize: rawconfig.Colorize,
	}.Print()
	return nil
}
