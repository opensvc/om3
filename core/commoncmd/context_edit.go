package commoncmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/util/file"
)

type (
	CmdContextEdit struct {
		Discard bool
		Recover bool
	}
)

func NewCmdContextEdit() *cobra.Command {
	var options CmdContextEdit

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a context",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()

	FlagDiscard(flags, &options.Discard)
	FlagRecover(flags, &options.Recover)
	return cmd
}

func (t *CmdContextEdit) Run() error {

	srcDir := clientcontext.ConfigFilename + ".json"
	src, err := homedir.Expand(srcDir)
	if err != nil {
		return err
	}
	mode := file.EditModeNormal
	if t.Discard {
		mode = file.EditModeDiscard
	}
	if t.Recover {
		mode = file.EditModeRecover
	}

	validate := func(dst string) error {
		f, err := os.ReadFile(dst)
		if err != nil {
			return err
		}
		if json.Valid(f) {
			return nil
		}
		return fmt.Errorf("invalid json format")
	}

	if err := file.Edit(src, mode, validate); err != nil {
		return err
	}

	return nil
}
