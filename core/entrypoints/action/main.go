package action

import (
	"fmt"
	"opensvc.com/opensvc/core/api/getevent"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/entrypoints/monitor"
	"os"
)

type (
	// Action switches between local, remote or async mode for a command action
	Action struct {
		//
		// ObjectSelector expands into a selection of objects to execute
		// the action on.
		//
		ObjectSelector string

		//
		// NodeSelector expands into a selection of nodes to execute the
		// action on.
		//
		NodeSelector string

		//
		// Local routes the action to the CRM instead of remoting it via
		// orchestration or remote execution.
		//
		Local bool

		//
		// DefaultIsLocal makes actions not explicitely Local nor remoted
		// via NodeSelector be treated as local (CRM level).
		//
		DefaultIsLocal bool

		//
		// Action is the name of the action as passed to the command line
		// interface.
		//
		Action string

		//
		// PostFlags is the dataset submited in the POST /{object|node}_action
		// api handler to execute the action remotely.
		//
		PostFlags map[string]interface{}

		//
		// Flags is the command flags as parsed by cobra. This is the struct
		// passed to the object method on local execution.
		//
		Flags interface{}

		//
		// Target is the node or object state the daemons should orchestrate
		// to reach.
		//
		Target string

		//
		// Watch runs a event-driven monitor on the selected objects after
		// setting a new target. So the operator can see the orchestration
		// unfolding.
		//
		Watch bool

		//
		// Format controls the output data format.
		// <empty>   => human readable format
		// json      => json machine readable format
		// flat      => flattened json (<k>=<v>) machine readable format
		// flat_json => same as flat (backward compat)
		//
		Format string

		//
		// Color activates the colorization of outputs
		// auto => yes if os.Stdout is a tty
		// yes
		// no
		//
		Color string

		//
		// Server bypasses the agent api requester automatic selection. It
		// Accepts a uri where the scheme can be:
		// raw   => jsonrpc
		// http  => http/2 cleartext (over unix domain socket only)
		// https => http/2 with TLS
		// tls   => http/2 with TLS
		//
		Server string
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
	switch {
	case o.NodeSelector != "":
		t.doRemote()
	case o.Local || o.DefaultIsLocal:
		t.doLocal()
	case o.Target != "":
		t.doAsync()
	case !client.WantContext():
		t.doLocal()
	default:
		// post action on context endpoint
		t.doRemote()
	}
	if o.Watch {
		m := monitor.New()
		m.SetColor(o.Color)
		m.SetFormat(o.Format)
		cli, err := client.New(client.URL(o.Server))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		getter := getevent.New(*cli, o.ObjectSelector, true)
		m.DoWatch(getter, os.Stdout)
	}
}
