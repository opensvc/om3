package cluster

import (
	"fmt"
	"io"
)

func wArbitrators(w io.Writer, data Data, info *dataInfo) {
	if len(info.arbitrators) == 0 {
		return
	}
	fmt.Fprintln(w, title("Arbitrators", data))
	fmt.Fprintln(w, info.empty)
}
