package monitor

//go:generate mockgen -source=main.go -destination=../mock_monitor/main.go

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
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
	Do() (<-chan event.Event, error)
}

// Do function renders the cluster status
func (m *T) Do(getter Getter, out io.Writer) error {
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

func (m *T) doOneShot(data cluster.Status, clear bool, out io.Writer) {
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

func (m *T) DoWatch(statusGetter Getter, evReader event.ReadCloser, out io.Writer) error {
	return m.watch(statusGetter, evReader, out)
}

func patchedStatus(b []byte, p jsondelta.Patch) ([]byte, *cluster.Status, error) {
	newB, err := p.Apply(b)
	if err != nil {
		//_, _ = fmt.Fprintf(os.Stderr, "patches.Apply failure: %s, patch len: %d, patch:%+v\n", err, len(p), p)
		return nil, nil, err
	}
	data := cluster.Status{}
	if err := json.Unmarshal(newB, &data); err != nil {
		return nil, nil, err
	}
	return newB, &data, nil
}

func (m *T) watch(statusGetter Getter, evReader event.ReadCloser, out io.Writer) error {
	var (
		b    []byte
		data *cluster.Status
		err  error

		errC   = make(chan error)
		eventC = make(chan *event.Event, 100)
		dataC  = make(chan *cluster.Status)

		patchById = make(map[uint64][]jsondelta.Operation)
		nextId    uint64

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
				errC <- fmt.Errorf("event read error %s", err)
				return
			}
			eventC <- ev
		}
	}()

	b, err = statusGetter.Get()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	wg.Add(1)
	go func(d *cluster.Status) {
		defer wg.Done()
		m.doOneShot(*d, true, out)
		// show data when new data published on dataC
		for d := range dataC {
			//_, _ = fmt.Fprintf(os.Stderr, "doOneShot %v\n", d.Cluster.Node[hostname.Hostname()].Status.Gen)
			m.doOneShot(*d, true, out)
		}
	}(data)

	defer close(dataC)

	ticker := time.NewTicker(displayInterval)
	defer ticker.Stop()
	var patchId uint64
	for {
		select {
		case err := <-errC:
			return err
		case e := <-eventC:
			switch e.Kind {
			case "DataUpdated":
				var evData []byte = e.Data
				if len(e.Data) == 0 {
					return fmt.Errorf("unexpected empty patch from event '%v'", e)
				} else if patch, err := jsondelta.NewPatch(evData); err == nil {
					patchId++
					patchById[patchId] = patch
				} else {
					return fmt.Errorf("can't create patch for '%s' id %d' error:'%s' data: '%s'", e.Kind, e.ID, err, e.Data)
				}
			default:
				// drop other eventC
			}
		case <-ticker.C:
			if len(patchById) > 0 {
				sortIds := make([]uint64, 0)
				for i := range patchById {
					sortIds = append(sortIds, i)
				}
				sort.Slice(sortIds, func(i, j int) bool { return sortIds[i] < sortIds[j] })
				patches := make([]jsondelta.Operation, 0)
				if nextId == 0 {
					// init nextId from first patch ev.ID
					nextId = sortIds[0]
				}
				for _, id := range sortIds {
					if id > nextId {
						return fmt.Errorf("break %d != %d, sortIds:%v", id, nextId, sortIds)
					} else if id < nextId && (nextId-id) > 20 {
						return fmt.Errorf("reset break %d != %d, sortIds:%v", id, nextId, sortIds)
					}
					nextId++
					patches = append(patches, patchById[id]...)
					delete(patchById, id)
				}
				if len(patches) > 0 {
					//_, _ = fmt.Fprintf(os.Stderr, "apply patches len: %d nextId: %d patch set ids: %v from available ids: %v\n",
					//	len(patches), nextId, idToDelete, sortIds)
					b, data, err = patchedStatus(b, patches)
					if err != nil {
						return fmt.Errorf("can't apply patch %s", err)
					}
					dataC <- data
				}
			}
		}
	}
}
