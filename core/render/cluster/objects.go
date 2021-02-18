package cluster

import (
	"fmt"
	"io"
)

func wObjects(w io.Writer, data Data, info *dataInfo) {
	fmt.Fprintln(w, title("Objects", data))
}
