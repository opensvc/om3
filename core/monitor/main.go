package monitor

//go:generate mockgen -source=main.go -destination=../mock_monitor/main.go

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/inancgumus/screen"

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

// Do function renders the cluster status
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

func handleEvent(b *[]byte, e event.Event) (err error) {
	patch := jsondelta.NewPatch(e.Data)
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
		Data:          data.WithSelector(m.selector),
		HumanRenderer: human,
		Colorize:      rawconfig.Colorize,
	}.Sprint()

	if clear {
		screen.Clear()
		screen.MoveTopLeft()
	}
	_, _ = fmt.Fprint(out, s)
}

func (m T) DoWatch(statusGetter Getter, eventGetter EventGetter, out io.Writer) error {
	for {
		if err := m.watch(statusGetter, eventGetter, out); err != nil {
			return err
		}
		// unexpected: avoid fast looping
		time.Sleep(1 * time.Second)
	}
}

func patchedStatus(b []byte, p jsondelta.Patch) ([]byte, *cluster.Status, error) {
	newB, err := p.Apply(b)
	if err != nil {
		//_, _ = fmt.Fprintf(os.Stderr, "patches.Apply failure: %s\npatch len: %d\n", err, len(p))
		//_, _ = fmt.Fprintf(os.Stderr, "patches: %+v\n", p)
		//_, _ = fmt.Fprintf(os.Stderr, "data to patch: %s\n", b)
		return nil, nil, err
	}
	data := cluster.Status{}
	if err := json.Unmarshal(newB, &data); err != nil {
		//_, _ = fmt.Fprintf(os.Stderr, "unmarshal data %s\ndocument: %s", err, b)
		return nil, nil, err
	}
	return newB, &data, nil
}

func (m T) watch(statusGetter Getter, eventGetter EventGetter, out io.Writer) error {
	var (
		b        []byte
		data     *cluster.Status
		err      error
		events   chan event.Event
		dataChan = make(chan *cluster.Status)

		patchById = make(map[string][]jsondelta.Operation)
		nextId    uint64

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
		m.doOneShot(*d, true, out)
		// show data when new data published on dataChan
		for d := range dataChan {
			//_, _ = fmt.Fprintf(os.Stderr, "doOneShot %v\n", d.Cluster.Node[hostname.Hostname()].Status.Gen)
			m.doOneShot(*d, true, out)
		}
	}(data)

	ticker := time.NewTicker(displayInterval)
	defer ticker.Stop()
	for {
		select {
		case e, ok := <-events:
			if !ok {
				_, _ = fmt.Fprintf(os.Stderr, "no more events\n")
				return nil
			}
			switch e.Kind {
			case "patch", "full":
				s := strconv.FormatUint(e.ID, 10)
				patchById[s] = jsondelta.NewPatch(e.Data)
			}
		case <-ticker.C:
			if len(patchById) > 0 {
				sortIds := make([]uint64, 0)
				for s := range patchById {
					id, err := strconv.ParseUint(s, 10, 64)
					if err != nil {
						continue
					}
					sortIds = append(sortIds, id)
				}
				sort.Slice(sortIds, func(i, j int) bool { return sortIds[i] < sortIds[j] })
				patches := make([]jsondelta.Operation, 0)
				if nextId == 0 {
					// init nextId from first patch ev.ID
					nextId = sortIds[0]
				}
				idToDelete := make([]string, 0)
				for _, id := range sortIds {
					if id > nextId {
						_, _ = fmt.Fprintf(os.Stderr, "break %d != %s, sortIds:%v\n",
							id, nextId, sortIds)
						return nil
					} else if id < nextId && (nextId-id) > 20 {
						_, _ = fmt.Fprintf(os.Stderr, "reset break %d != %s, sortIds:%v\n",
							id, nextId, sortIds)
						return nil
					}
					nextId++
					s := strconv.FormatUint(id, 10)
					patches = append(patches, patchById[s]...)
					idToDelete = append(idToDelete, s)
					delete(patchById, s)
				}
				if len(patches) > 0 {
					//_, _ = fmt.Fprintf(os.Stderr, "patches len: %d nextId: %d patch set ids: %v from availables ids: %v\n",
					//	len(patches), nextId, idToDelete, sortIds)
					b, data, err = patchedStatus(b, patches)
					if err != nil {
						return err
					}
					dataChan <- data
				}
			}
		}
	}
}
