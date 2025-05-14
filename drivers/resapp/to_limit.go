package resapp

import (
	"github.com/opensvc/om3/util/capexec"
)

func (t *T) toCaps() capexec.T {
	xo := capexec.T{}
	//xo.LoadPG(*t.GetPG())
	pg := *t.GetPG()
	if pg.ID != "" {
		xo.PGID = &pg.ID
	}
	if t.User != "" {
		xo.User = &t.User
	}
	if t.Group != "" {
		xo.Group = &t.Group
	}
	xo.LoadLimit(t.Limit)
	return xo
}
