package oxcmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"

	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/util/file"
)

type (
	CmdContextEdit struct {
		Discard bool
		Recover bool
	}
)

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
