package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/monitor"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/oxcmd"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/logging"
	"github.com/rivo/tview"
	"github.com/rs/zerolog"
)

type (
	viewId    int
	viewStack []viewId

	Position struct {
		row int
		col int
	}

	PoolData struct {
		pool.Status
		Node string
	}

	CreateTableOptions struct {
		title        string
		titles       []string
		elementsList [][]string
		selectables  []int
		capture      func(event *tcell.EventKey, v *tview.Table) *tcell.EventKey
	}

	App struct {
		*monitor.Frame

		user       string
		eventCount uint64

		stack viewStack

		app      *tview.Application
		top      *tview.TextView
		head     *tview.Table
		errs     *tview.TextView
		textView *tview.TextView
		keys     *tview.Table
		objects  *tview.Table
		flex     *tview.Flex
		command  *tview.InputField
		contexts *tview.List

		client       *client.T
		streamClient *client.T

		lastDraw time.Time

		selectedElement         string
		previousSelectedElement string
		position                Position

		events       chan event.Event
		onEventView  atomic.Bool
		stopEvents   bool
		eventsCtx    context.Context
		eventsCancel context.CancelFunc
		isInEventView atomic.Bool

		viewPath naming.Path
		viewNode string
		viewKey  string
		viewRID  string

		lastUpdatedAt time.Time

		firstInstanceCol int
		firstObjectRow   int

		maxRetries      int
		displayInterval time.Duration

		selectedNodes     map[string]any
		selectedPaths     map[string]any
		selectedInstances map[[2]string]any
		selectedRIDs      map[[3]string]any

		errC     chan error
		restartC chan error
		exitFlag atomic.Bool

		logCloser io.Closer
	}

	getter interface {
		Get() ([]byte, error)
	}
)

const (
	viewObject viewId = iota
	viewConfig
	viewKey
	viewKeys
	viewInstance
	viewLog
	viewPool
	viewPoolVolume
	viewNetwork
	viewNetworkIpList
	viewEvents
	viewLast // marker, not a real view
)

var (
	colorNone      = tcell.ColorNone
	colorSelected  = tcell.ColorDarkSlateGray
	colorTitle     = tcell.ColorGray
	colorHead      = tcell.ColorSteelBlue
	colorHead2     = tcell.ColorOlive
	colorHead3     = tcell.ColorCrimson
	colorInput     = tcell.ColorSteelBlue
	colorHighlight = tcell.ColorWhite

	forceUpdate    = true
	updateIfChange = false
)

type Options struct {
	Selector string
}

func Run(options *Options) error {
	app := NewApp()
	if options != nil {
		if options.Selector != "" {
			app.Frame.Selector = options.Selector
		}
	}
	return app.Run()
}

func (t viewStack) String() string {
	l := []string{
		viewObject.String(),
	}
	for _, v := range t {
		l = append(l, v.String())
	}
	return strings.Join(l, " > ")
}

func (t *App) updateHead() {
	type titler interface{ GetTitle() string }
	if t.flex.GetItemCount() < 2 {
		return
	}
	primitive := t.flex.GetItem(1)
	box, ok := primitive.(titler)
	if !ok {
		return
	}
	conn := func() string {
		endpoint := ""
		if t.client != nil {
			endpoint = t.client.Hostname()
		}
		switch {
		case t.user == "" && endpoint == "":
			return ""
		case t.user != "" && endpoint == "":
			return fmt.Sprintf("%s@%s (uds)", t.user, hostname.Hostname())
		default:
			return fmt.Sprintf("%s@%s", t.user, endpoint)
		}
	}
	title := box.GetTitle()
	t.head.SetCell(0, 0, tview.NewTableCell(conn()).SetBackgroundColor(colorHead3))
	t.head.SetCell(0, 1, tview.NewTableCell("").SetBackgroundColor(colorHead).SetTextColor(colorHead3))
	t.head.SetCell(0, 2, tview.NewTableCell(t.Frame.Current.Cluster.Config.Name).SetBackgroundColor(colorHead))
	t.head.SetCell(0, 3, tview.NewTableCell("").SetBackgroundColor(colorHead2).SetTextColor(colorHead))
	t.head.SetCell(0, 4, tview.NewTableCell(title).SetBackgroundColor(colorHead2))
	t.head.SetCell(0, 5, tview.NewTableCell("").SetTextColor(colorHead2))
}

func (t viewId) String() string {
	switch t {
	case viewObject:
		return "objects"
	case viewConfig:
		return "configuration"
	case viewKey:
		return "key"
	case viewKeys:
		return "keys"
	case viewInstance:
		return "instance"
	case viewLog:
		return "log"
	case viewPool:
		return "pool"
	case viewPoolVolume:
		return "pool volume"
	case viewNetwork:
		return "network"
	case viewNetworkIpList:
		return "network ip list"
	default:
		return ""
	}
}

func (t *App) push(v viewId) {
	t.stack = append(t.stack, v)
}

func (t *App) pop() viewId {
	n := len(t.stack)
	if n == 0 {
		return viewObject
	}
	v := t.stack[n-1]
	t.stack = t.stack[:n-1]
	return v
}

func (t *App) focus() viewId {
	n := len(t.stack)
	if n == 0 {
		return viewObject
	}
	return t.stack[n-1]
}

func NewApp() *App {
	return &App{
		stack:            make([]viewId, 0),
		firstInstanceCol: 5,
		maxRetries:       600,
		displayInterval:  500 * time.Millisecond,
		Frame: &monitor.Frame{
			Selector: "*/svc/*",
			Sections: []string{},
		},
		selectedNodes:     make(map[string]any),
		selectedPaths:     make(map[string]any),
		selectedInstances: make(map[[2]string]any),
		selectedRIDs:      make(map[[3]string]any),
		errC:              make(chan error),
		restartC:          make(chan error),
		events:            make(chan event.Event, 100),
	}
}

func (t *App) resetSelected() int {
	t.selectedElement = t.previousSelectedElement
	t.previousSelectedElement = ""
	switch t.focus() {
	case viewInstance:
		n := len(t.selectedRIDs)
		t.resetSelectedRIDs()
		return n
	default:
		return 0
	}
}

func (t *App) resetAllSelected() {
	t.resetSelectedNodes()
	t.resetSelectedPaths()
	t.resetSelectedInstances()
	t.resetSelectedRIDs()
}

func (t *App) initHeadTextView() {
	t.head = tview.NewTable()
	t.head.SetBorder(false)
}

func (t *App) initErrsTextView() {
	t.errs = tview.NewTextView()
	t.errs.SetBorder(false)
}

func (t *App) viewPrimitive(v viewId) tview.Primitive {
	switch v {
	case viewConfig, viewInstance, viewKey, viewLog, viewEvents:
		return t.textView
	case viewKeys:
		return t.keys
	default:
		return t.objects
	}
}

func (t *App) initApp() {
	t.initHeadTextView()
	t.initObjectsTable()
	t.initErrsTextView()

	t.app = tview.NewApplication()
	t.flex = tview.NewFlex().SetDirection(tview.FlexRow)
	t.flex.AddItem(t.head, 1, 0, false)
	t.updateHead()
	t.flex.AddItem(t.objects, 0, 1, true)
	t.app.SetRoot(t.flex, true)

	t.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if t.command != nil {
			return event
		}
		switch event.Key() {
		case tcell.KeyESC:
			if n := t.resetSelected(); n > 0 {
				return nil
			}
			t.back()
		}
		switch event.Rune() {
		case ':':
			t.onRuneColumn(event)
			return nil
		case 'c':
			t.onRuneC(event)
		case 'e':
			t.onRuneE(event)
		case 'h':
			t.onRuneH(event)
		case 'l':
			t.nav(viewLog)
		case 'q':
			t.stop()
		case 'r':
			t.onRuneR(event)
		}
		return event
	})
}

func (t *App) init() error {
	t.initApp()

	monitor.InitColor()

	return nil
}

func (t *App) listContexts() {
	cfg, err := clientcontext.Load()
	if err != nil {
		t.errorf("%s", err)
	}

	v := tview.NewTable()
	v.SetSelectable(true, false)
	v.SetTitle("connect to")

	v.SetCell(0, 0, tview.NewTableCell("CONTEXT").SetTextColor(colorTitle).SetSelectable(false))
	v.SetCell(0, 1, tview.NewTableCell("ENDPOINT").SetTextColor(colorTitle).SetSelectable(false))
	v.SetCell(0, 2, tview.NewTableCell("USER").SetTextColor(colorTitle).SetSelectable(false))
	v.SetCell(0, 3, tview.NewTableCell("NAMESPACE").SetTextColor(colorTitle).SetSelectable(false))

	row := 1
	v.SetCell(row, 0, tview.NewTableCell("").SetSelectable(true))
	v.SetCell(row, 1, tview.NewTableCell("localhost").SetSelectable(false))
	v.SetCell(row, 2, tview.NewTableCell("root").SetSelectable(false))
	v.SetCell(row, 3, tview.NewTableCell(os.Getenv("OSVC_NAMESPACE")).SetSelectable(false))

	contexts := make([]string, len(cfg.Contexts))
	i := 0
	for context := range cfg.Contexts {
		contexts[i] = context
		i++
	}
	sort.Strings(contexts)
	for _, name := range contexts {
		data := cfg.Contexts[name]
		row++
		selectable := true
		cluster, clusterOk := cfg.Clusters[data.ClusterRefName]
		_, userOk := cfg.Users[data.UserRefName]
		if clusterOk {
			v.SetCell(row, 1, tview.NewTableCell(cluster.Server).SetSelectable(false))
		} else {
			v.SetCell(row, 1, tview.NewTableCell("-").SetSelectable(false))
			selectable = false
		}
		if userOk {
			v.SetCell(row, 2, tview.NewTableCell(data.UserRefName).SetSelectable(false))
		} else {
			v.SetCell(row, 2, tview.NewTableCell("-").SetSelectable(false))
			selectable = false
		}
		v.SetCell(row, 0, tview.NewTableCell(name).SetSelectable(selectable))
		v.SetCell(row, 3, tview.NewTableCell(data.Namespace).SetSelectable(false))
	}

	v.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			t.flex.Clear()
			t.flex.AddItem(t.head, 1, 0, false)
			t.flex.AddItem(t.objects, 0, 1, true)
			t.app.SetFocus(t.objects)
			t.updateHead()
		}
		return event
	})
	v.SetSelectedFunc(func(row, col int) {
		c := v.GetCell(row, col).Text
		os.Setenv("OSVC_CONTEXT", c)
		if cli, err := client.New(); err != nil {
			t.errorf("%s", err)
		} else if resp, err := cli.GetAuthWhoAmIWithResponse(context.Background()); err != nil {
			t.errorf("%s", err)
			t.listContexts()
		} else if resp.StatusCode() == http.StatusOK {
			t.client = cli
			if streamClient, err := client.New(client.WithTimeout(0)); err != nil {
				t.errorf("new stream client: %s", err)
				t.listContexts()
			} else {
				t.streamClient = streamClient
				t.user = resp.JSON200.Name
				t.reconnect()
				t.flex.Clear()
				t.flex.AddItem(t.head, 1, 0, false)
				t.flex.AddItem(t.objects, 0, 1, true)
				t.app.SetFocus(t.objects)
				t.updateHead()
			}
		}
	})

	t.flex.Clear()
	t.flex.AddItem(t.head, 1, 0, false)
	t.flex.AddItem(v, 0, 1, true)
	t.app.SetFocus(v)
	t.updateHead()
}

func (t *App) Run() error {
	if err := t.init(); err != nil {
		return err
	}
	go t.runEventReader()
	go t.initContext()
	return t.app.Run()
}

func (t *App) initContext() {
	if cli, err := client.New(); err != nil {
		t.errorf("%s", err)
	} else if resp, err := cli.GetAuthWhoAmIWithResponse(context.Background()); err != nil {
		t.errorf("%s", err)
		t.listContexts()
	} else if resp.StatusCode() == http.StatusOK {
		t.client = cli
		if streamClient, err := client.New(client.WithTimeout(0)); err != nil {
			t.errorf("new stream client: %s", err)
			t.listContexts()
		} else {
			t.streamClient = streamClient
			t.user = resp.JSON200.Name
			t.reconnect()
		}
	} else {
		t.listContexts()
	}
}

func (t *App) runEventReader() {
	<-t.restartC
	for {
		evReader, err := t.streamClient.NewGetEvents().SetSelector(t.Selector).GetReader()
		if err != nil {
			t.errorf("new reader: %s", err)
			if t.exitFlag.Load() {
				return
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		statusGetter := t.client.NewGetClusterStatus().SetSelector(t.Selector)
		err = t.do(statusGetter, evReader)
		_ = evReader.Close()
		if t.exitFlag.Load() {
			return
		}
		if err != nil {
			t.errorf("do with reader: %s", err)
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (t *App) do(statusGetter getter, evReader event.ReadCloser) error {
	var (
		b    []byte
		data *clusterdump.Data
		err  error

		eventC = make(chan event.Event, 100)
		dataC  = make(chan *clusterdump.Data)

		nextEventID uint64

		wg = sync.WaitGroup{}
	)

	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(eventC)

		for {
			ev, err := evReader.Read()
			if err != nil {
				t.errC <- err
				return
			}
			eventC <- *ev
		}
	}()

	//t.infof("get daemon status")
	b, err = statusGetter.Get()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	cdata := msgbus.NewClusterData(data)
	wg.Add(1)
	go func(d *clusterdump.Data) {
		defer wg.Done()
		t.Current = *d
		t.Nodename = data.Daemon.Nodename
		t.app.QueueUpdateDraw(func() {
			t.updateHead()
			t.updateObjects()
		})
		// show data when new data published on dataC
		for d := range dataC {
			t.Current = *d
			t.Nodename = data.Daemon.Nodename
			t.eventCount++
			t.app.QueueUpdateDraw(func() {
				// TODO: detect if t.updateInstanceView and t.updateConfigView need to be called (config mtime change, ...)
				t.updateHead()
				switch t.focus() {
				case viewInstance:
					t.updateInstanceView()
				case viewConfig:
					t.updateConfigView()
				case viewKeys:
					t.updateKeysView()
				case viewPool:
					t.updatePoolList(updateIfChange)
				case viewPoolVolume:
					t.updatePoolVolume(t.selectedElement)
				case viewNetwork:
					t.updateNetworkList()
				case viewNetworkIpList:
					t.updateNetworkIpList(t.selectedElement)
				case viewEvents:
					t.updateEventsView()
				default:
					t.updateObjects()
				}
			})
		}
	}(data.DeepCopy())

	defer close(dataC)

	ticker := time.NewTicker(t.displayInterval)
	defer ticker.Stop()
	changes := false
	for {
		select {
		case <-t.restartC:
			_ = evReader.Close()
		case err := <-t.errC:
			return err
		case e := <-eventC:
			if t.isInEventView.Load() {
				t.events <- e
			}
			if nextEventID == 0 {
				nextEventID = e.ID
			} else if e.ID != nextEventID {
				err := fmt.Errorf("broken event chain: received event id %d, expected %d", e.ID, nextEventID)
				t.errorf("%s", err)
				return err
			}
			nextEventID++
			changes = true
			msg, err := msgbus.EventToMessage(e)
			if err != nil {
				t.errorf("EventToMessage event id %d %s error: %s", e.ID, e.Kind, err)
				continue
			}
			cdata.ApplyMessage(msg)
		case <-ticker.C:
			if changes {
				dataC <- cdata.DeepCopy()
				changes = false
			} else if t.focus() == viewObject {
				t.app.QueueUpdateDraw(func() {
					s := fmt.Sprint(time.Now().Truncate(time.Second).Sub(t.lastDraw.Truncate(time.Second)))
					t.objects.SetCell(2, 1, tview.NewTableCell(s).SetSelectable(false))
				})
			}
		}
	}
}

func (t *App) paths() []string {
	paths := make([]string, len(t.Current.Cluster.Object))
	i := 0
	for path := range t.Current.Cluster.Object {
		paths[i] = path
		i += 1
	}
	sort.Strings(paths)
	return paths

}

func (t *App) toggleRID(path, node, rid string) bool {
	key := [3]string{path, node, rid}
	if _, ok := t.selectedRIDs[key]; ok {
		delete(t.selectedRIDs, key)
		return false
	} else {
		t.selectedRIDs[key] = nil
		t.resetSelectedPaths()
		t.resetSelectedNodes()
		t.resetSelectedInstances()
		return true
	}
}

func (t *App) toggleInstance(path, node string) bool {
	key := [2]string{path, node}
	if _, ok := t.selectedInstances[key]; ok {
		delete(t.selectedInstances, key)
		return false
	} else {
		t.selectedInstances[key] = nil
		t.resetSelectedPaths()
		t.resetSelectedNodes()
		t.resetSelectedRIDs()
		return true
	}
}

func (t *App) togglePath(key string) bool {
	if _, ok := t.selectedPaths[key]; ok {
		delete(t.selectedPaths, key)
		return false
	} else {
		t.selectedPaths[key] = nil
		t.resetSelectedInstances()
		t.resetSelectedNodes()
		t.resetSelectedRIDs()
		return true
	}
}

func (t *App) toggleNode(key string) bool {
	if _, ok := t.selectedNodes[key]; ok {
		delete(t.selectedNodes, key)
		return false
	} else {
		t.selectedNodes[key] = nil
		t.resetSelectedInstances()
		t.resetSelectedPaths()
		t.resetSelectedRIDs()
		return true
	}
}

func (t *App) resetSelectedRIDs() {
	if len(t.selectedRIDs) == 0 {
		return
	}
	t.selectedRIDs = make(map[[3]string]any)
	if t.flex.GetItemCount() < 2 {
		return
	}
	primitive := t.flex.GetItem(1)
	table, ok := primitive.(*tview.Table)
	if !ok {
		return
	}
	if table.GetCell(0, 0).Text != "RID" {
		return
	}
	for i := 1; i < table.GetRowCount(); i += 1 {
		table.GetCell(i, 0).SetBackgroundColor(colorNone)
	}
}

func (t *App) resetSelectedNodes() {
	if len(t.selectedNodes) == 0 {
		return
	}
	t.selectedNodes = make(map[string]any)
	for j := t.firstInstanceCol; j < t.objects.GetColumnCount(); j += 1 {
		t.objects.GetCell(0, j).SetBackgroundColor(colorNone)
	}
}

func (t *App) resetSelectedInstances() {
	if len(t.selectedInstances) == 0 {
		return
	}
	t.selectedInstances = make(map[[2]string]any)
	for i := 1; i < t.objects.GetRowCount(); i += 1 {
		for j := t.firstInstanceCol; j < t.objects.GetColumnCount(); j += 1 {
			t.objects.GetCell(i, j).SetBackgroundColor(colorNone)
		}
	}
}

func (t *App) resetSelectedPaths() {
	if len(t.selectedPaths) == 0 {
		return
	}
	t.selectedPaths = make(map[string]any)
	for i := 1; i < t.objects.GetRowCount(); i += 1 {
		t.objects.GetCell(i, 0).SetBackgroundColor(colorNone)
	}
}

func (t *App) isResourceSelected(path, node, rid string) bool {
	_, ok := t.selectedRIDs[[3]string{path, node, rid}]
	return ok
}

func (t *App) isInstanceSelected(path, node string) bool {
	_, ok := t.selectedInstances[[2]string{path, node}]
	return ok
}

func (t *App) isPathSelected(path string) bool {
	_, ok := t.selectedPaths[path]
	return ok
}

func (t *App) isNodeSelected(node string) bool {
	_, ok := t.selectedNodes[node]
	return ok
}

func (t *App) onRuneColumn(event *tcell.EventKey) {
	clean := func() {
		t.flex.RemoveItem(t.command)
		t.command = nil
		t.app.SetFocus(t.flex.GetItem(1))
	}
	if t.command != nil {
		clean()
		return
	}
	clusterAction := func(action string) {
		switch action {
		case "freeze":
			t.actionClusterFreeze()
		case "unfreeze", "thaw":
			t.actionClusterUnfreeze()
		}
	}
	objectAction := func(action string, paths map[string]any) {
		switch action {
		case "stop":
			t.actionStop(paths)
		case "start":
			t.actionStart(paths)
		case "provision":
			t.actionProvision(paths)
		case "unprovision":
			t.actionUnprovision(paths)
		case "freeze":
			t.actionFreeze(paths)
		case "unfreeze", "thaw":
			t.actionUnfreeze(paths)
		case "switch":
			t.actionSwitch(paths)
		case "giveback":
			t.actionGiveback(paths)
		case "abort":
			t.actionAbort(paths)
		case "purge":
			t.actionPurge(paths)
		case "delete":
			t.actionDelete(paths)
		case "restart":
			t.actionRestart(paths)
		default:
			t.errorf("unknown object action: %s", action)
		}
	}
	resourceAction := func(args []string, keys map[[3]string]any) {
		switch args[0] {
		case "stop":
			t.actionResourceStop(keys)
		case "start":
			t.actionResourceStart(keys)
		case "provision":
			t.actionResourceProvision(keys)
		case "unprovision":
			t.actionResourceUnprovision(keys)
		case "restart":
			t.actionResourceRestart(keys)
		case "run":
			t.actionResourceRun(keys)
		case "enable":
			t.actionResourceEnable(keys)
		case "disable":
			t.actionResourceDisable(keys)
		default:
			t.errorf("unknown resource action: %s", args[0])
		}
	}
	instanceAction := func(action string, keys map[[2]string]any) {
		switch action {
		case "clear":
			t.actionInstanceClear(keys)
		case "stop":
			t.actionInstanceStop(keys)
		case "start":
			t.actionInstanceStart(keys)
		case "provision":
			t.actionInstanceProvision(keys)
		case "unprovision":
			t.actionInstanceUnprovision(keys)
		case "freeze":
			t.actionInstanceFreeze(keys)
		case "unfreeze", "thaw":
			t.actionInstanceUnfreeze(keys)
		case "restart":
			t.actionInstanceRestart(keys)
		case "refresh":
			t.actionInstanceRefresh(keys)
		case "switch":
			t.actionInstanceSwitch(keys)
		//	case "clear":
		//		t.actionInstanceClear(keys)
		default:
			t.errorf("unknown instance action: %s", action)
		}
	}
	nodeAction := func(args []string, nodes map[string]any) {
		switch args[0] {
		case "daemon":
			if len(args) < 2 {
				return
			}
			switch args[1] {
			case "restart":
				t.actionNodeDaemonRestart(nodes)
			}
		case "freeze":
			t.actionNodeFreeze(nodes)
		case "unfreeze", "thaw":
			t.actionNodeUnfreeze(nodes)
		case "drain":
			t.actionNodeDrain(nodes)
		default:
			t.errorf("unknown node action: %s", args[0])
		}
	}
	t.command = tview.NewInputField().
		SetLabel(":").
		SetFieldWidth(0).
		SetFieldBackgroundColor(colorInput).
		SetAutocompleteFunc(func(currentText string) (entries []string) {
			completions := t.getCompletions(currentText)
			slices.Sort(completions)
			return completions
		}).
		SetDoneFunc(func(key tcell.Key) {
			text := strings.TrimSpace(t.command.GetText())
			var action string
			args := strings.Fields(text)
			/*if len(args) == 0 {
				clean()
				return
			}*/
			if len(args) > 0 {
				action = args[0]
			}

			switch key {
			case tcell.KeyEnter:
				switch action {
				case "quit", "q":
					t.stop()
				case "connect":
					t.listContexts()
					clean()
				case "filter":
					if len(args) < 2 {
						t.errorf("not enough arguments: filter <expression>")
						return
					}
					t.setFilter(args[1])
					clean()
					return
				case "go":
					if len(args) < 2 {
						t.errorf("not enough arguments: go <to>")
						return
					}
					switch args[1] {
					case "sec":
						t.setFilter("*/sec/*")
						clean()
						return
					case "cfg":
						t.setFilter("*/cfg/*")
						clean()
						return
					case "usr":
						t.setFilter("*/usr/*")
						clean()
						return
					case "svc":
						t.setFilter("*/svc/*")
						clean()
						return
					case "vol":
						t.setFilter("*/vol/*")
						clean()
						return
					case "pool":
						t.nav(viewPool)
						clean()
						return
					case "network", "net":
						t.nav(viewNetwork)
						clean()
						return
					}
				case "do":
					if len(args) < 2 {
						t.errorf("not enough arguments: do <action>")
						return
					}
					action = args[1]
					switch {
					case len(t.selectedRIDs) > 0:
						resourceAction(args[1:], t.selectedRIDs)
					case len(t.selectedPaths) > 0:
						objectAction(action, t.selectedPaths)
					case len(t.selectedInstances) > 0:
						instanceAction(action, t.selectedInstances)
					case len(t.selectedNodes) > 0:
						nodeAction(args[1:], t.selectedNodes)
					default:
						row, col := t.objects.GetSelection()
						switch {
						case t.focus() == viewInstance && row > 1:
							if table, ok := t.flex.GetItem(2).(*tview.Table); ok {
								row, col := table.GetSelection()
								rid := table.GetCell(row, col).Text
								selection := make(map[[3]string]any)
								selection[[3]string{t.viewPath.String(), t.viewNode, rid}] = nil
								resourceAction(args[1:], selection)
							}
						case row == 0 && col == 1:
							clusterAction(action)
						case row == 0 && col >= t.firstInstanceCol:
							node := t.objects.GetCell(row, col).Text
							selection := make(map[string]any)
							selection[node] = nil
							nodeAction(args[1:], selection)
						case row >= t.firstObjectRow && col == 0:
							path := t.objects.GetCell(row, 0).Text
							selection := make(map[string]any)
							selection[path] = nil
							objectAction(action, selection)
						case row >= t.firstObjectRow && col >= t.firstInstanceCol:
							path := t.objects.GetCell(row, 0).Text
							node := t.objects.GetCell(0, col).Text
							selection := make(map[[2]string]any)
							selection[[2]string{path, node}] = nil
							instanceAction(action, selection)
						}
					}
					clean()
				}
			case tcell.KeyEscape:
				clean()
			}
		})
	t.flex.RemoveItem(t.errs)
	t.flex.AddItem(t.command, 1, 0, true)
	t.app.SetFocus(t.command)
}

func (t *App) setFilter(s string) {
	t.Frame.Selector = s
	t.reconnect()
}

func (t *App) actionNodeDaemonRestart(nodes map[string]any) {
	ctx := context.Background()
	for node, _ := range nodes {
		_, _ = t.client.PostDaemonRestart(ctx, node)
	}
}

func (t *App) actionNodeDrain(nodes map[string]any) {
	ctx := context.Background()
	for node, _ := range nodes {
		_, _ = t.client.PostPeerActionDrainWithResponse(ctx, node)
	}
}

func (t *App) actionNodeFreeze(nodes map[string]any) {
	ctx := context.Background()
	for node, _ := range nodes {
		_, _ = t.client.PostPeerActionFreezeWithResponse(ctx, node, nil)
	}
}

func (t *App) actionNodeUnfreeze(nodes map[string]any) {
	ctx := context.Background()
	for node, _ := range nodes {
		_, _ = t.client.PostPeerActionUnfreezeWithResponse(ctx, node, nil)
	}
}

func (t *App) actionAbort(paths map[string]any) {
	ctx := context.Background()
	for path := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionAbortWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionRestart(paths map[string]any) {
	ctx := context.Background()
	for path := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionRestartWithResponse(ctx, p.Namespace, p.Kind, p.Name, api.PostObjectActionRestart{})
	}
}

func (t *App) actionInstanceRefresh(keys map[[2]string]any) {
	ctx := context.Background()
	for key := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionStatusWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceRestart(keys map[[2]string]any) {
	ctx := context.Background()
	for key := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionRestartWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceStart(keys map[[2]string]any) {
	ctx := context.Background()
	for key := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionStartWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceClear(keys map[[2]string]any) {
	ctx := context.Background()
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceClearWithResponse(ctx, node, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionInstanceStop(keys map[[2]string]any) {
	ctx := context.Background()
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionStopWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceProvision(keys map[[2]string]any) {
	ctx := context.Background()
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionProvisionWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceUnprovision(keys map[[2]string]any) {
	ctx := context.Background()
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionUnprovisionWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceFreeze(keys map[[2]string]any) {
	ctx := context.Background()
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionFreezeWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceUnfreeze(keys map[[2]string]any) {
	ctx := context.Background()
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostInstanceActionUnfreezeWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, nil)
	}
}

func (t *App) actionInstanceSwitch(keys map[[2]string]any) {
	ctx := context.Background()
	m := make(map[string][]string)
	for key, _ := range keys {
		path := key[0]
		node := key[1]
		if l, ok := m[path]; ok {
			l = append(l, node)
			m[path] = l
		} else {
			l = append([]string{}, node)
			m[path] = l
		}
	}
	for path, nodes := range m {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		body := api.PostObjectActionSwitch{
			Destination: nodes,
		}
		_, _ = t.client.PostObjectActionSwitchWithResponse(ctx, p.Namespace, p.Kind, p.Name, body)
	}
}

func groupByInstance(in map[[3]string]any) map[[2]string][]string {
	out := make(map[[2]string][]string)
	for key := range in {
		k := [2]string{key[0], key[1]}
		rid := key[2]
		if l, ok := out[k]; ok {
			l = append(l, rid)
			out[k] = l
		} else {
			l := []string{rid}
			out[k] = l
		}
	}
	return out
}

func (t *App) actionResourceRestart(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		rid := strings.Join(rids, ",")
		params := api.PostInstanceActionRestartParams{Rid: &rid}
		_, _ = t.client.PostInstanceActionRestartWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceEnable(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		unset := make(api.InQueryUnsets, len(rids))
		for i, rid := range rids {
			unset[i] = rid + ".disable"
		}
		params := api.PatchObjectConfigParams{Unset: &unset}
		_, _ = t.client.PatchObjectConfigWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceDisable(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		set := make(api.InQuerySets, len(rids))
		for i, rid := range rids {
			set[i] = rid + ".disable=true"
		}
		params := api.PatchObjectConfigParams{Set: &set}
		_, _ = t.client.PatchObjectConfigWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceRun(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		rid := strings.Join(rids, ",")
		confirm := true
		params := api.PostInstanceActionRunParams{Rid: &rid, Confirm: &confirm}
		_, _ = t.client.PostInstanceActionRunWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceProvision(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		rid := strings.Join(rids, ",")
		params := api.PostInstanceActionProvisionParams{Rid: &rid}
		_, _ = t.client.PostInstanceActionProvisionWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceUnprovision(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}

		rid := strings.Join(rids, ",")
		params := api.PostInstanceActionUnprovisionParams{Rid: &rid}
		_, _ = t.client.PostInstanceActionUnprovisionWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceStart(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		rid := strings.Join(rids, ",")
		params := api.PostInstanceActionStartParams{Rid: &rid}
		_, _ = t.client.PostInstanceActionStartWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionResourceStop(keys map[[3]string]any) {
	ctx := context.Background()
	for key, rids := range groupByInstance(keys) {
		path := key[0]
		node := key[1]
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		rid := strings.Join(rids, ",")
		params := api.PostInstanceActionStopParams{Rid: &rid}
		_, _ = t.client.PostInstanceActionStopWithResponse(ctx, node, p.Namespace, p.Kind, p.Name, &params)
	}
}

func (t *App) actionStop(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionStopWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionPurge(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionPurgeWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionDelete(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionDeleteWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionStart(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionStartWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionProvision(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionProvisionWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionUnprovision(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionUnprovisionWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionFreeze(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionFreezeWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionUnfreeze(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionUnfreezeWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionSwitch(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		body := api.PostObjectActionSwitch{}
		_, _ = t.client.PostObjectActionSwitchWithResponse(ctx, p.Namespace, p.Kind, p.Name, body)
	}
}

func (t *App) actionGiveback(paths map[string]any) {
	ctx := context.Background()
	for path, _ := range paths {
		p, err := naming.ParsePath(path)
		if err != nil {
			continue
		}
		_, _ = t.client.PostObjectActionGivebackWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	}
}

func (t *App) actionClusterFreeze() {
	ctx := context.Background()
	_, _ = t.client.PostClusterActionFreezeWithResponse(ctx)
}

func (t *App) actionClusterUnfreeze() {
	ctx := context.Background()
	_, _ = t.client.PostClusterActionUnfreezeWithResponse(ctx)
}

func (t *App) onRuneH(event *tcell.EventKey) {
	help := `
 Command mode Shortcuts
 
   :                    Enter command mode
   ESC                  Exit command mode
   Enter                Apply command to the selected cells
 
 Selection Shortcuts
 
   Up,Right,Down,Left   Move cursor
   SPACE                Select the cell
   ESC                  Reset selection
   Ctrl-a               Invert object selection
 
 Misc Shortcuts
 
   c                    Show object configuration
   h                    Show this help
   l                    Show node, object or instance logs
   q                    Quit
   r                    Refresh the instance status
   Enter                Show the detailed instance status
   ESC                  Close popup

 Commands:

   connect              Connect to another cluster

   do <action>

     cluster actions:
       freeze, unfreeze

     object actions:
       abort, delete, freeze, giveback, provision, purge, start, stop, switch,
       unfreeze, unprovision, restart  

     instance actions:
       clear, delete, freeze, provision, refresh, start, stop, switch,
       unfreeze, unprovision  

     resource actions:
       disable, enable, provision, run, start, stop, unprovision  

     node actions:
       drain freeze, unfreeze

   go <to>

     sec, cfg, vol, pool, net

   filter <expression>
`
	savedItem := t.flex.GetItem(1)
	savedFocus := t.app.GetFocus()

	v := tview.NewTextView().
		SetScrollable(true).
		SetDynamicColors(true).
		SetText(help)
	v.SetBorder(true).
		SetTitle("Help").
		SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyESC:
				t.flex.RemoveItem(v)
				t.flex.AddItem(t.head, 1, 0, false)
				t.flex.AddItem(savedItem, 0, 1, true)
				t.app.SetFocus(savedFocus)
			}
			return event
		})
	t.flex.Clear()
	t.flex.AddItem(v, 0, 1, true)
	t.app.SetFocus(v)
}

func (t *App) updateLogTextView() {
	title := func() string {
		switch {
		case !t.viewPath.IsZero() && t.viewNode != "":
			return fmt.Sprintf("%s@%s log", t.viewPath, t.viewNode)
		case !t.viewPath.IsZero():
			return fmt.Sprintf("%s log", t.viewPath)
		case t.viewNode != "":
			return fmt.Sprintf("%s log", t.viewNode)
		default:
			return ""
		}
	}

	t.textView.SetTitle(title())
	t.textView.SetDynamicColors(true)
	t.textView.SetChangedFunc(func() {
		t.textView.ScrollToEnd()
	})
	t.textView.Clear()

	lines := 50
	follow := true
	log := t.streamClient.NewGetLogs(t.viewNode).
		//SetFilters(nil).
		SetLines(&lines).
		SetFollow(&follow)
	if !t.viewPath.IsZero() {
		l := naming.Paths{t.viewPath}.StrSlice()
		log.SetPaths(&l)
	}
	reader, err := log.GetReader()
	if err != nil {
		t.errorf("%s", err)
		return
	}
	t.logCloser = reader

	w := zerolog.NewConsoleWriter()
	w.Out = tview.ANSIWriter(t.textView)
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.FormatLevel = logging.FormatLevel
	w.FormatFieldName = func(i any) string { return "" }
	w.FormatFieldValue = func(i any) string { return "" }
	w.FormatMessage = func(i any) string {
		return rawconfig.Colorize.Bold(i)
	}

	go func() {
		for {
			event, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			} else if errors.Is(err, context.Canceled) {
				break
			} else if err != nil {
				t.errorf("%s", err)
				break
			}
			rec, err := streamlog.NewEvent(event.Data)
			if err != nil {
				t.errorf("%s", err)
				break
			}
			switch s := rec.M["JSON"].(type) {
			case string:
				_, _ = w.Write([]byte(s))
			}
		}
	}()
}

func (t *App) getConfigUpdatedAt() time.Time {
	path := t.viewPath.String()
	for _, nodeData := range t.Current.Cluster.Node {
		if instanceData, ok := nodeData.Instance[path]; ok {
			if instanceData.Config == nil {
				continue
			}
			return instanceData.Config.UpdatedAt
		}
	}
	return time.Time{}
}

func (t *App) skipIfConfigNotUpdated() bool {
	if updatedAt := t.getConfigUpdatedAt(); updatedAt.IsZero() {
		t.errorf("instance config disappeared")
		return true
	} else if !updatedAt.After(t.lastUpdatedAt) {
		return true
	} else {
		t.lastUpdatedAt = updatedAt
		return false
	}
}

func (t *App) skipIfInstanceNotUpdated() bool {
	if nodeData, ok := t.Current.Cluster.Node[t.viewNode]; !ok {
		t.errorf("node config disappeared")
		return true
	} else if instanceData, ok := nodeData.Instance[t.viewPath.String()]; !ok {
		t.errorf("instance config disappeared")
		return true
	} else if instanceData.Config != nil && instanceData.Config.UpdatedAt.After(t.lastUpdatedAt) {
		t.lastUpdatedAt = instanceData.Config.UpdatedAt
		return false
	} else if instanceData.Status != nil && instanceData.Status.UpdatedAt.After(t.lastUpdatedAt) {
		t.lastUpdatedAt = instanceData.Status.UpdatedAt
		return false
	}
	// no change, skip
	return true
}

func (t *App) onRuneE(event *tcell.EventKey) {
	t.app.Suspend(func() {
		row, col := t.objects.GetSelection()
		switch {
		case !t.viewPath.IsZero() && t.viewKey != "":
			cmd := oxcmd.CmdObjectKeyEdit{
				Name: t.viewKey,
			}
			if err := cmd.DoRemote(t.viewPath, t.client); err != nil {
				t.errorf("%s", err)
			}
		case !t.viewPath.IsZero():
			cmd := oxcmd.CmdObjectConfigEdit{}
			if err := cmd.DoRemote(t.viewPath, t.client); err != nil {
				t.errorf("%s", err)
			}
		case t.viewNode != "":
			cmd := oxcmd.CmdNodeConfigEdit{}
			if err := cmd.DoRemote(t.viewNode, t.client); err != nil {
				t.errorf("%s", err)
			}
		case row == 0 && col == 1:
			cmd := oxcmd.CmdObjectConfigEdit{}
			if err := cmd.DoRemote(naming.Cluster, t.client); err != nil {
				t.errorf("%s", err)
			}
		}
	})
}

func (t *App) onRuneR(event *tcell.EventKey) {
	switch {
	case t.viewPath.IsZero():
		return
	case t.viewNode == "":
		return
	}
	key := [2]string{
		t.viewPath.String(),
		t.viewNode,
	}
	t.actionInstanceRefresh(map[[2]string]any{
		key: nil,
	})
}

func (t *App) onRuneC(event *tcell.EventKey) {
	t.initTextView()
	t.updateConfigView()
	t.nav(viewConfig)
}

func (t *App) updateConfigView() {
	row, col := t.objects.GetSelection()
	switch {
	case !t.viewPath.IsZero():
		t.updateObjectConfigView()
	case t.viewNode != "":
		t.updateNodeConfigView()
	case row == 0 && col == 1:
		t.updateClusterConfigView()
	}
}

func (t *App) updateClusterConfigView() {
	if !t.lastUpdatedAt.IsZero() {
		return
	}
	t.lastUpdatedAt = time.Now()
	resp, err := t.client.GetClusterConfigFileWithResponse(context.Background())
	if err != nil {
		return
	}
	if resp.StatusCode() != http.StatusOK {
		return
	}

	text := tview.TranslateANSI(string(resp.Body))
	t.textView.SetDynamicColors(false)
	t.textView.Clear()
	t.textView.SetTitle("cluster configuration")
	fmt.Fprint(t.textView, text)
}

func (t *App) updateNodeConfigView() {
	if !t.lastUpdatedAt.IsZero() {
		return
	}
	t.lastUpdatedAt = time.Now()
	resp, err := t.client.GetNodeConfigFileWithResponse(context.Background(), t.viewNode)
	if err != nil {
		return
	}
	if resp.StatusCode() != http.StatusOK {
		return
	}

	text := tview.TranslateANSI(string(resp.Body))
	t.textView.SetDynamicColors(false)
	t.textView.SetTitle(fmt.Sprintf("%s configuration", t.viewNode))
	t.textView.Clear()
	fmt.Fprint(t.textView, text)
}

func (t *App) updateObjectConfigView() {
	if t.skipIfConfigNotUpdated() {
		return
	}
	resp, err := t.client.GetObjectConfigFileWithResponse(context.Background(), t.viewPath.Namespace, t.viewPath.Kind, t.viewPath.Name)
	if err != nil {
		return
	}
	if resp.StatusCode() != http.StatusOK {
		return
	}

	text := tview.TranslateANSI(string(resp.Body))
	t.textView.SetDynamicColors(false)
	t.textView.SetTitle(fmt.Sprintf("%s configuration", t.viewPath.String()))
	t.textView.Clear()
	fmt.Fprint(t.textView, text)
}

func (t *App) infof(format string, args ...any) {
	t.printf(tcell.ColorGray, format, args...)
}

func (t *App) warnf(format string, args ...any) {
	t.printf(tcell.ColorOrange, format, args...)
}

func (t *App) errorf(format string, args ...any) {
	t.printf(tcell.ColorRed, format, args...)
}

func (t *App) printf(color tcell.Color, format string, args ...any) {
	t.flex.AddItem(t.errs, 1, 0, false)
	t.errs.Clear()
	t.errs.SetBackgroundColor(color)
	fmt.Fprintf(t.errs, format, args...)
	time.AfterFunc(5*time.Second, func() {
		t.flex.RemoveItem(t.errs)
	})
}

func (t *App) nav(to viewId) {
	from := t.focus()
	t.push(to)
	if to == from {
		return
	}
	t.navFromTo(from, to)
}

func (t *App) back() {
	from := t.pop()
	to := t.focus()
	t.navFromTo(from, to)
}

func (t *App) navFromTo(from, to viewId) {
	t.flex.Clear()
	t.flex.AddItem(t.head, 1, 0, false)
	t.lastUpdatedAt = time.Time{}
	t.position = Position{row: 0, col: 0}
	switch from {
	case viewObject:
	case viewLog:
		t.textView.SetChangedFunc(nil)
		t.textView = nil
		if t.logCloser != nil {
			t.logCloser.Close()
		}
	case viewConfig, viewInstance, viewKey:
		t.textView = nil
	case viewKeys:
		t.keys = nil
	case viewEvents:
		t.textView = nil
		if t.eventsCancel != nil {
			t.eventsCancel()
		}
		t.isInEventView.Store(false)
	}
	switch to {
	case viewLog:
		t.initTextView()
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
		t.updateLogTextView()
	case viewConfig:
		t.initTextView()
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
	case viewKey:
		t.initTextView()
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
		t.updateKeyTextView()
	case viewInstance:
		t.initTextView()
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
		t.updateInstanceView()
	case viewKeys:
		t.initKeysTable()
		t.flex.AddItem(t.keys, 0, 1, true)
		t.app.SetFocus(t.keys)
		t.updateKeysView()
	case viewObject:
		t.flex.AddItem(t.objects, 0, 1, true)
		t.app.SetFocus(t.objects)
		t.updateObjects()
	case viewNetwork:
		t.updateNetworkList()
	case viewPool:
		t.updatePoolList(forceUpdate)
	case viewNetworkIpList:
		t.updateNetworkIpList(t.selectedElement)
	case viewPoolVolume:
		t.updatePoolVolume(t.selectedElement)
	case viewEvents:
		t.isInEventView.Store(true)
		t.initTextView()
		t.initEventsView()
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
		t.updateEventsView()
	}
	t.updateHead()
	t.flex.AddItem(t.errs, 1, 0, false)
}

func (t *App) createTable(creator CreateTableOptions) {
	v := tview.NewTable()
	v.SetSelectable(true, true)
	v.SetTitle(creator.title)
	for i, title := range creator.titles {
		v.SetCell(0, i, tview.NewTableCell(title).SetTextColor(colorTitle).SetSelectable(false))
	}

	v.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			t.stop()
		}
		if creator.capture != nil {
			return creator.capture(event, v)
		}
		return event
	})

	v.SetSelectionChangedFunc(func(row, column int) {
		t.position = Position{row: row, col: column}
	})

	for i, elements := range creator.elementsList {
		row := i + 1
		for j, element := range elements {
			selectable := j == 0 || (creator.selectables != nil && slices.Contains(creator.selectables, j))
			v.SetCell(row, j, tview.NewTableCell(element).SetSelectable(selectable))
		}
	}

	v.Select(t.position.row, t.position.col)

	t.flex.Clear()
	t.flex.AddItem(t.head, 1, 0, false)
	t.flex.AddItem(v, 0, 1, true)
	t.app.SetFocus(v)
	t.updateHead()
}

func (t *App) selectedString() string {
	if t.viewNode == "" {
		if t.viewPath.IsZero() {
			return ""
		} else {
			return t.viewPath.String()
		}
	} else {
		if t.viewPath.IsZero() {
			return t.viewNode
		} else {
			return t.viewPath.String() + "@" + t.viewNode
		}
	}
}

func (t *App) initTextView() {
	if t.textView != nil {
		return
	}
	v := tview.NewTextView()
	v.SetScrollable(true)
	v.SetBorder(false)
	t.textView = v
	return
}

func (t *App) reconnect() {
	t.restartC <- nil
}

func (t *App) stop() {
	t.exitFlag.Store(true)
	select {
	case t.errC <- nil:
	default:
	}

	close(t.events)
	t.app.Stop()
}
