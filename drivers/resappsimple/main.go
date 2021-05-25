package resappsimple

import (
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resapp"
)

// T is the driver structure.
type T struct {
	resapp.T
	Kill string `json:"kill"`
}

func New() resource.Driver {
	return &T{}
}

func init() {
	resource.Register(driverGroup, driverName, New)
}

// Start the Resource
func (t T) Start() error {
	t.Log().Debug().Msg("Start()")
	appStatus := t.Status()
	if appStatus == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}
	cmd := t.GetCmd(t.StartCmd, "start")
	t.Log().Info().Msgf("starting %s", cmd.String())
	err := cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return driverGroup.String()
}
