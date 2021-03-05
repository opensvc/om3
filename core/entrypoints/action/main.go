package action

import (
	"opensvc.com/opensvc/core/entrypoints/monitor"
)

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
		Server         string
	}

	// actioner is a interface implemented for node and object.
	actioner interface {
		doRemote()
		doLocal()
		doAsync()
		options() Action
	}
)

// Do is the switch method between local, remote or async mode.
// If Watch is set, end up starting a monitor on the selected objects.
func Do(t actioner) {
	o := t.options()
	if o.Local {
		t.doLocal()
	} else if o.NodeSelector != "" {
		t.doRemote()
	} else {
		t.doAsync()
	}
	if o.Watch {
		m := monitor.New()
		m.SetWatch(true)
		m.SetColor(o.Color)
		m.SetFormat(o.Format)
		m.SetSelector(o.ObjectSelector)
		m.SetServer(o.Server)
		m.Do()
	}
}
