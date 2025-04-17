package oxcmd

import "fmt"

type (
	CmdObjectEnter struct {
		ObjectSelector string
		RID            string
	}
)

func (t *CmdObjectEnter) Run(kind string) error {
	return fmt.Errorf("TODO")
}
