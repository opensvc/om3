package resfsdir

import (
	"fmt"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
)

const (
	driverGroup = drivergroup.FS
	driverName  = "directory"
)

var Keywords = []keywords.Keyword{
	{
		Option:   "path",
		Scopable: true,
		Required: true,
		Text:     "The fullpath of the directory to create.",
	},
	{
		Option:   "user",
		Scopable: true,
		Example:  "root",
		Text:     "The user that should be owner of the directory. Either in numeric or symbolic form.",
	},
	{
		Option:   "group",
		Scopable: true,
		Example:  "sys",
		Text:     "The group that should be owner of the directory. Either in numeric or symbolic form.",
	},
	{
		Option:   "perm",
		Scopable: true,
		Example:  "1777",
		Text:     "The permissions the directory should have. A string representing the octal permissions.",
	},
}

type (
	T struct {
		resource.T
	}
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() resource.Manifest {
	return resource.Manifest{
		Group:    driverGroup,
		Name:     driverName,
		Keywords: Keywords,
	}
}

func (t T) Start() error {
	return nil
}

func (t T) Stop() error {
	return nil
}

func (t T) Status() status.T {
	return status.NotApplicable
}

func (t T) Label() string {
	return fmt.Sprintf("dir %s", t.path())
}

func (t T) path() string {
	return ""
}
