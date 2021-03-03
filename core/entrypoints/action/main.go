package action

import "opensvc.com/opensvc/core/entrypoints/monitor"

type (
	// Action switches between local, remote or async mode for a command action
	Action struct {
		ObjectSelector string
		NodeSelector   string
		Local          bool
		Action         string
		Method         string
		Target         string
		Watch          bool
		Format         string
		Color          string
	}

	// Actioner is a interface implemented for node and object.
	Actioner interface {
		Do()
		DoRemote()
		DoAsync()
		Options() Action
	}
)

func do(t Actioner) {
	o := t.Options()
	if o.NodeSelector != "" {
		t.DoRemote()
	} else {
		t.DoAsync()
	}
	if o.Watch {
		m := monitor.New()
		m.SetWatch(true)
		m.SetColor(o.Color)
		m.SetFormat(o.Format)
		m.SetSelector(o.ObjectSelector)
		m.Do()
	}
}
