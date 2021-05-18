package resappunix

import (
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/app/resappbase"
)

// T is the driver structure for app unix & linux.
type T struct {
	resappbase.T
	Path     path.T   `json:"path"`
	Nodes    []string `json:"nodes"`
	StartCmd string   `json:"start"`
	StopCmd  string   `json:"stop"`
	CheckCmd string   `json:"check"`
}

func (t T) Abort() bool {
	return false
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
	err := resappbase.Command(t.StopCmd).Run()
	if err != nil {
		return err
	}
	return nil
}

// Status evaluates and display the Resource status and logs
func (t *T) Status() status.T {
	t.Log().Debug().Msgf("Status() running %s", t.CheckCmd)
	err := resappbase.Command(t.CheckCmd).Run()
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
