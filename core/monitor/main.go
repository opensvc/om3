package monitor

//go:generate mockgen -source=main.go -destination=../mock_monitor/main.go

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/inancgumus/screen"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/msgbus"
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
// human-readable.
func (m *T) SetFormat(v string) {
	m.format = v
}

// SetSelector sets the selector option. Default is "*".
func (m *T) SetSelector(v string) {
	m.selector = v
}

// SetSections sets the sections option, controlling which sections to render
// (threads, nodes, arbitrators, objects). Defaults to an empty list, interpreted
// as all sections.
func (m *T) SetSections(v []string) {
	m.sections = v
}

// SetSectionsFromExpression sets the sections option, parsing a string representation
// of a section list, using comma as separator.
func (m *T) SetSectionsFromExpression(s string) {
	v := make([]string, 0)
	if s != "" {
		v = strings.Split(s, ",")
	}
	m.SetSections(v)
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
	Do() (<-chan event.Event, error)
}

// Do function renders the cluster status
func (m *T) Do(getter Getter, out io.Writer) error {
	var err error
	b, err := getter.Get()
	if err != nil {
		return err
	}
	var data cluster.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	m.doOneShot(data, false, out)
	return nil
}

func (m *T) doOneShot(data cluster.Data, clear bool, out io.Writer) {
	human := func() string {
		f := cluster.Frame{
			Current:  data,
			Sections: m.sections,
		}
		return f.Render()
	}

	s, err := output.Renderer{
		Output:        m.format,
		Color:         m.color,
		Data:          data.WithSelector(m.selector),
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Sprint()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if clear {
		screen.Clear()
		screen.MoveTopLeft()

		// Clearing is used by the watch mode.
		// In this case we want to see the date as a proof of activity.
		_, _ = fmt.Fprintf(out, "%s\n\n", time.Now().Format(time.RFC1123))
	}
	_, _ = fmt.Fprint(out, s)
}

func (m *T) DoWatch(statusGetter Getter, evReader event.ReadCloser, out io.Writer) error {
	return m.watch(statusGetter, evReader, out)
}

func (m *T) watch(statusGetter Getter, evReader event.ReadCloser, out io.Writer) error {
	var (
		b    []byte
		data *cluster.Data
		err  error

		errC   = make(chan error)
		eventC = make(chan event.Event, 100)
		dataC  = make(chan *cluster.Data)

		nextEvId uint64

		displayInterval = 500 * time.Millisecond

		wg = sync.WaitGroup{}
	)

	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(eventC)
		defer close(errC)

		for {
			ev, err := evReader.Read()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "evReader.Read error %s\n", err)
				errC <- fmt.Errorf("event read error %s", err)
				return
			}
			eventC <- *ev
		}
	}()

	b, err = statusGetter.Get()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	cdata := msgbus.NewClusterData(data)
	wg.Add(1)
	go func(d *cluster.Data) {
		defer wg.Done()
		m.doOneShot(*d, true, out)
		// show data when new data published on dataC
		for d := range dataC {
			m.doOneShot(*d, true, out)
		}
	}(data.DeepCopy())

	defer close(dataC)

	ticker := time.NewTicker(displayInterval)
	defer ticker.Stop()
	changes := false
	for {
		select {
		case err := <-errC:
			return err
		case e := <-eventC:
			if nextEvId == 0 {
				nextEvId = e.ID
			} else if e.ID != nextEvId {
				_, _ = fmt.Fprintf(os.Stderr, "receive broken event id %d %s\n", e.ID, e.Kind)
				return fmt.Errorf("broken event id sequence wanted %d got %d", nextEvId, e.ID)
			}
			nextEvId++
			changes = true
			if msg, err := msgbus.EventToMessage(e); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "EventToMessage event id %d %s error %s\n", e.ID, e.Kind, err)
				continue
			} else if err := cdata.ApplyMessage(msg); err != nil {
				return err
			}
		case <-ticker.C:
			if changes {
				dataC <- cdata.DeepCopy()
				changes = false
			}
		}
	}
}
