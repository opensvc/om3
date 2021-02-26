package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/inancgumus/screen"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	// Type is a monitor renderer instance. It stores the rendering options.
	Type struct {
		watch    bool
		color    string
		format   string
		selector string
		sections []string
		nodes    []string
	}
)

// CmdLong factorizes the long desc text defined by commands invoking a Monitor.
const CmdLong = `Color convention:
  red     issue
  orange  warning

Object Flags:
  !       Warning
  ^       Placement non-optimal

Instance Flags:
  O       Up
  S       Standby up
  X       Down
  s       Standby down
  !       Warning
  P       Unprovisioned
  *       Frozen
  ^       Placement leader
  #       DRP instance
`

// New allocates a monitor.
func New() Type {
	return Type{
		watch:    false,
		selector: "*",
		color:    "auto",
		format:   "auto",
	}
}

// SetSelector sets the selector option
func (m *Type) SetSelector(v string) {
	m.selector = v
}

// SetWatch sets the watch option. Default is false. If true, listen to events
// and re-render new cluster data.
func (m *Type) SetWatch(v bool) {
	m.watch = v
}

// SetColor sets the color option. Default is "auto", interpreted as colored if
// the terminal as a tty.
func (m *Type) SetColor(v string) {
	m.color = v
}

// SetFormat sets the rendering format option. Default is "auto", interpreted as
// human readable.
func (m *Type) SetFormat(v string) {
	m.format = v
}

// SetSections sets the sections option, controlling which sections to render
// (threads, nodes, arbitrators, objects). Defaults to an empty list, interpreted
// as all sections.
func (m *Type) SetSections(v []string) {
	m.sections = v
}

// SetNodes sets the nodes option, controlling which node columns to render.
// Defaults to an empty list, interpreted as all nodes.
func (m *Type) SetNodes(v []string) {
	m.nodes = v
}

// Do renders the cluster status
func (m Type) Do() {
	var (
		api client.API
		err error
	)
	api, err = client.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if m.watch {
		if err = m.doWatch(api); err != nil {
			fmt.Println(err)
		}
		return
	}
	opts := client.NewDaemonStatusOptions()
	opts.ObjectSelector = m.selector
	data, err := api.DaemonStatus(*opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	m.doOneshot(data, false)
}

func (m Type) doWatch(api client.API) error {
	var (
		data   cluster.Status
		ok     bool
		err    error
		evt    event.Event
		events chan []byte
	)
	handle := api.NewGetEvents()
	handle.Full = true
	handle.ObjectSelector = m.selector
	events, err = handle.DoRaw()
	if err != nil {
		return err
	}
	b, ok := <-events
	if !ok {
		return errors.New("event channel unexpectedly closed")
	}
	evt, err = event.DecodeFromJSON(b)
	if err != nil {
		return err
	}
	b = *evt.Data
	json.Unmarshal(*evt.Data, &data)
	m.doOneshot(data, true)
	for e := range events {
		evt, err := event.DecodeFromJSON(e)
		if err != nil {
			fmt.Fprintln(os.Stderr, err, string(e))
			continue
		}
		err = handleEvent(&b, evt)
		if err != nil {
			fmt.Fprintln(os.Stderr, err, string(e))
			return err
		}
		json.Unmarshal(b, &data)
		m.doOneshot(data, true)
	}
	return nil
}

func handleEvent(b *[]byte, e event.Event) error {
	var err error
	switch e.Kind {
	case "event":
		return nil
	case "patch", "full":
		patch := jsondelta.NewPatch(*e.Data)
		*b, err = patch.Apply(*b)
		if err != nil {
			return err
		}
	default:
		// unexpected: avoid fast looping
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (m Type) doOneshot(data cluster.Status, clear bool) {
	human := func() string {
		f := cluster.Frame{
			Current:  data,
			Sections: m.sections,
		}
		return f.Render()
	}

	s := output.Switch(m.format, m.color, data, human)
	if clear {
		screen.Clear()
		screen.MoveTopLeft()
	}
	fmt.Print(s)
}
