package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/version"
)

func CmdNodeVersion() {
	v := version.Version()
	fmt.Println(v)
}
