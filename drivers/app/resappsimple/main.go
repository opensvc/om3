package resappsimple

import (
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/app"
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	return &T{}
}

func (t T) Abort() bool {
	return false
}

// Start the Resource
func (t T) Start() error {
	t.Log().Debug().Msg("Start()")
	appStatus := t.Status()
	if appStatus == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}
	t.Log().Info().Msgf("starting %s", t.StartCmd)
	err := app.Command(t.StartCmd).Start()
	if err != nil {
		return err
	}
	return nil
}

// Stop the Resource
func (t T) Stop() error {
	t.Log().Debug().Msg("Stop()")
	appStatus := t.Status()
	if appStatus == status.Down {
		t.Log().Info().Msg("already down")
		return nil
	}
	t.Log().Info().Msgf("running %s", t.StopCmd)
	err := app.Command(t.StopCmd).Run()
	if err != nil {
		return err
	}
	return nil
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	return driverGroup.String()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	t.Log().Debug().Msgf("Status() running %s", t.CheckCmd)
	err := app.Command(t.CheckCmd).Run()
	if err != nil {
		t.Log().Debug().Msg("status is down")
		return status.Down
	}
	t.Log().Debug().Msgf("status is up")
	return status.Up
}

func (t T) Provision() error {
	return nil
}

func (t T) Unprovision() error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}
