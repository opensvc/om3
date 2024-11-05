package resapp

import (
	"github.com/opensvc/om3/util/capexec"
)

func (t T) toCaps() capexec.T {
	xo := capexec.T{}
	//xo.LoadPG(*t.GetPG())
	xo.LoadLimit(t.Limit)
	return xo
}
