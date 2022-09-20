package monitor

//go:generate mockgen -source=main.go -destination=../mock_monitor/main.go

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/inancgumus/screen"
	"github.com/pkg/errors"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/event"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	// T is a monitor renderer instance. It stores the rendering options.
	T struct {
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
func New() T {
	return T{
		selector: "*",
		color:    "auto",
		format:   "auto",
	}
}

// SetColor sets the color option. Default is "auto", interpreted as colored if
// the terminal as a tty.
func (m *T) SetColor(v string) {
	m.color = v
}

// SetFormat sets the rendering format option. Default is "auto", interpreted as
// human readable.
func (m *T) SetFormat(v string) {
	m.format = v
}

// SetSections sets the sections option, controlling which sections to render
// (threads, nodes, arbitrators, objects). Defaults to an empty list, interpreted
// as all sections.
func (m *T) SetSections(v []string) {
	m.sections = v
}

// SetNodes sets the nodes option, controlling which node columns to render.
// Defaults to an empty list, interpreted as all nodes.
func (m *T) SetNodes(v []string) {
	m.nodes = v
}

type Getter interface {
	Get() ([]byte, error)
}

type EventBGetter interface {
	GetRaw() (chan []byte, error)
}

type EventGetter interface {
	Do() (chan event.Event, error)
}

// Do renders the cluster status
func (m T) Do(getter Getter, out io.Writer) error {
	var err error
	b, err := getter.Get()
	if err != nil {
		return err
	}
	var data cluster.Status
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	m.doOneShot(data, false, out)
	return nil
}

func (m T) DoWatch(eventGetter EventBGetter, out io.Writer) error {
	for {
		if err := m.watch(eventGetter, out); err != nil {
			return err
		}
		// unexpected: avoid fast looping
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (m T) watch(eventGetter EventBGetter, out io.Writer) error {
	var (
		data   cluster.Status
		ok     bool
		err    error
		evt    event.Event
		events chan []byte
	)
	events, err = eventGetter.GetRaw()
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
	if err := json.Unmarshal(*evt.Data, &data); err != nil {
		return err
	}
	m.doOneShot(data, true, out)
	for e := range events {
		evt, err := event.DecodeFromJSON(e)
		if err != nil {
			//log.Debug().Err(err).Msgf("decode event %v", e)
			continue
		}

		switch evt.Kind {
		case "event":
			continue
		case "patch", "full":
			// pass
		default:
			// unexpected: avoid fast looping
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if err := handleEvent(&b, evt); err != nil {
			return errors.Wrap(err, "handle event")
		}
		if err := json.Unmarshal(b, &data); err != nil {
			return errors.Wrap(err, "unmarshal event data")
		}
		m.doOneShot(data, true, out)
	}
	return nil
}

func handleEvent(b *[]byte, e event.Event) (err error) {
	patch := jsondelta.NewPatch(*e.Data)
	*b, err = patch.Apply(*b)
	return
}

func (m T) doOneShot(data cluster.Status, clear bool, out io.Writer) {
	human := func() string {
		f := cluster.Frame{
			Current:  data,
			Sections: m.sections,
		}
		return f.Render()
	}

	s := output.Renderer{
		Format:        m.format,
		Color:         m.color,
		Data:          data,
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Sprint()

	if clear {
		screen.Clear()
		screen.MoveTopLeft()
	}
	_, _ = fmt.Fprint(out, s)
}

func (m T) DoWatchDemo(statusGetter Getter, eventGetter EventGetter, out io.Writer) error {
	for {
		if err := m.watchdemo(statusGetter, eventGetter, out); err != nil {
			return err
		}
		// unexpected: avoid fast looping
		time.Sleep(1 * time.Second)
	}
}

func (m T) watchdemo(statusGetter Getter, eventGetter EventGetter, out io.Writer) error {
	var (
		b        []byte
		data     cluster.Status
		err      error
		events   chan event.Event
		dataChan = make(chan *cluster.Status)

		patches = make(jsondelta.Patch, 0)

		displayInterval = 500 * time.Millisecond
	)
	events, err = eventGetter.Do()
	if err != nil {
		return err
	}

	b, err = statusGetter.Get()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	go func(d *cluster.Status) {
		// show data when new data published on dataChan
		for d := range dataChan {
			m.doOneShot(*d, true, out)
		}
	}(&data)

	ticker := time.NewTicker(displayInterval)
	defer ticker.Stop()
	for {
		select {
		case e, ok := <-events:
			if !ok {
				fmt.Fprintf(os.Stderr, "no more events\n")
				return nil
			}
			switch e.Kind {
			case "patch", "full":
				patches = append(patches, jsondelta.NewPatch(*e.Data)...)
			}
		case <-ticker.C:
			if len(patches) > 0 {
				if newB, err := patches.Apply(b); err != nil {
					fmt.Fprintf(os.Stderr, "patches.Apply failure: %s\nlen(patches): %d\n", err, len(patches))
					fmt.Fprintf(os.Stderr, "patches: %+v\n", patches)
					fmt.Fprintf(os.Stderr, "b: %s\n", b)
					return err
				} else {
					b = newB
				}
				data := cluster.Status{}
				if err := json.Unmarshal(b, &data); err != nil {
					fmt.Fprintf(os.Stderr, "unmarshal data %s\ndata: %s", err, b)
					return err
				}
				patches = make(jsondelta.Patch, 0)
				dataChan <- &data
			}
		}
	}
}
