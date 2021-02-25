package cluster

import (
	"fmt"
)

func (f Frame) wArbitrators() {
	if len(f.info.arbitrators) == 0 {
		return
	}
	fmt.Fprintln(f.w, f.title("Arbitrators"))
	fmt.Fprintln(f.w, f.info.empty)
}
