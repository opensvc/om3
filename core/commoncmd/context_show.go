package commoncmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	CmdContextShow struct {
	}
)

func NewCmdContextShow() *cobra.Command {
	var options CmdContextShow

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the context configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	return cmd
}

func (t *CmdContextShow) Run() error {
	config, err := clientcontext.Load()
	if err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))

	return nil
}
