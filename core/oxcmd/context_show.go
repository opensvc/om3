package oxcmd

import (
	"encoding/json"
	"fmt"

	"github.com/opensvc/om3/v3/core/clientcontext"
)

type (
	CmdContextShow struct {
	}
)

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
