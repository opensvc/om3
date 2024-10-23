package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/event"
	"github.com/opensvc/om3/core/monitor"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/streamlog"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/rivo/tview"
	"github.com/rs/zerolog"
)

type (
	viewId    int
	viewStack []viewId

	App struct {
		*monitor.Frame

		eventCount uint64

		stack []viewId

		app      *tview.Application
		top      *tview.TextView
		errs     *tview.TextView
		textView *tview.TextView
		keys     *tview.Table
		objects  *tview.Table
		flex     *tview.Flex
		command  *tview.InputField

		client *client.T

		lastDraw time.Time

		viewPath naming.Path
		viewNode string

		firstInstanceCol int
		firstObjectRow   int

		maxRetries      int
		displayInterval time.Duration

		selectedNodes     map[string]any
		selectedPaths     map[string]any
		selectedInstances map[[2]string]any

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
)

var (
	colorNone      = tcell.ColorNone
	colorSelected  = tcell.ColorDarkSlateGray
	colorTitle     = tcell.ColorGray
	colorHighlight = tcell.ColorWhite

	spin    = []rune{'⠁', '⠂', '⠄', '⡀', '⢀', '⠠', '⠐', '⠈'}
	spinLen = len(spin)
)

func main() {
	if err := NewApp().Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		errC:              make(chan error),
		restartC:          make(chan error),
	}
}

func (t *App) resetAllSelected() {
	t.resetSelectedNodes()
	t.resetSelectedPaths()
	t.resetSelectedInstances()
}

func (t *App) initKeysTable() {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(fmt.Sprintf("%s keys", t.viewPath)).SetBorderPadding(1, 1, 1, 1)

	onEnter := func(event *tcell.EventKey) {
		row, col := table.GetSelection()
		if row == 0 {
			return
		}
		key := table.GetCell(row, col).Text
		resp, err := t.client.GetObjectKVStoreEntryWithResponse(context.Background(), t.viewPath.Namespace, t.viewPath.Kind, t.viewPath.Name, &api.GetObjectKVStoreEntryParams{
			Key: key,
		})
		if err != nil {
			t.errorf("%s", err)
			return
		}
		if resp.StatusCode() != http.StatusOK {
			t.errorf("status code: %s", resp.Status())
			return
		}

		t.initTextView()
		text := string(resp.Body)
		title := fmt.Sprintf("%s key %s", t.viewPath, key)
		t.textView.SetTitle(title)
		t.textView.Clear()
		fmt.Fprint(t.textView, text)
		t.nav(viewKey)
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
			table.SetSelectable(true, true)
		case tcell.KeyESC:
			t.back()
		case tcell.KeyEnter:
			onEnter(event)
			return nil // prevents the default select behaviour
		}
		switch event.Rune() {
		case 'c':
			t.onRuneC(event)
		case 'h':
			t.onRuneH(event)
		case 'l':
			t.onRuneL(event)
		case 's':
			t.onRuneS(event)
		case 'q':
			t.stop()
		case ':':
			t.onRuneColumn(event)
		default:
			return event
		}
		return nil
	})
	t.keys = table
}

func (t *App) initObjectsTable() {
	table := tview.NewTable()

	selectedFunc := func(row, column int) {
		cell := table.GetCell(row, column)
		path := table.GetCell(row, 0).Text
		node := table.GetCell(0, column).Text
		var selected *bool
		switch {
		case row < t.firstObjectRow-1:
		case row == t.firstObjectRow-1:
			v := t.toggleNode(node)
			selected = &v
		case column == 0:
			v := t.togglePath(path)
			selected = &v
		case column >= t.firstInstanceCol:
			v := t.toggleInstance(path, node)
			selected = &v
		}
		if selected != nil && *selected {
			cell.SetBackgroundColor(colorSelected)
		} else {
			cell.SetBackgroundColor(colorNone)
		}
	}

	table.SetSelectedFunc(selectedFunc)

	setSelection := func(table *tview.Table) {
		row, col := table.GetSelection()
		cell := table.GetCell(row, col)
		cell.SetBackgroundColor(colorSelected)
		table.SetCell(row, col, cell)
		selectedFunc(row, col)
	}

	selectAll := func() {
		for i := t.firstObjectRow; i < table.GetRowCount(); i++ {
			selectedFunc(i, 0)
		}
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
			table.SetSelectable(true, true)
		case tcell.KeyESC:
			t.resetSelectedNodes()
			t.resetSelectedPaths()
			t.resetSelectedInstances()
		case tcell.KeyCtrlA:
			selectAll()
		case tcell.KeyEnter:
			t.onEnter(event)
			return nil // prevents the default select behaviour
		}
		switch event.Rune() {
		case ' ':
			setSelection(table)
		case 'c':
			t.onRuneC(event)
		case 'h':
			t.onRuneH(event)
		case 'l':
			t.onRuneL(event)
		case 's':
			t.onRuneS(event)
		case 'q':
			t.stop()
		case ':':
			t.onRuneColumn(event)
		default:
			return event
		}
		return nil
	})
	t.objects = table
}

func (t *App) initErrsTextView() {
	t.errs = tview.NewTextView()
	t.errs.SetBorder(false)
}

func (t *App) initApp() {
	t.initObjectsTable()
	t.initErrsTextView()

	t.app = tview.NewApplication()
	t.flex = tview.NewFlex().SetDirection(tview.FlexRow)
	t.flex.AddItem(t.objects, 0, 1, true)
	t.app.SetRoot(t.flex, true)
}

func (t *App) init() error {
	if len(os.Args) > 1 {
		t.Frame.Selector = os.Args[1]
	}
	t.initApp()

	if cli, err := client.New(client.WithTimeout(0)); err != nil {
		return err
	} else {
		t.client = cli
	}

	monitor.InitColor()

	return nil
}

func (t *App) Run() error {
	if err := t.init(); err != nil {
		return err
	}
	go t.runEventReader()
	return t.app.Run()
}

func (t *App) runEventReader() {
	for {
		evReader, err := t.client.NewGetEvents().SetSelector(t.Selector).GetReader()
		if err != nil {
			t.errorf("%s", err)
			if t.exitFlag.Load() {
				return
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}

		statusGetter := t.client.NewGetDaemonStatus().SetSelector(t.Selector)
		err = t.do(statusGetter, evReader)
		_ = evReader.Close()
		if t.exitFlag.Load() {
			return
		}
		if err != nil {
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
				err = fmt.Errorf("event queue read error: %w", err)
				t.errorf("%s", err)
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
			t.updateObjects()
		})
		// show data when new data published on dataC
		for d := range dataC {
			t.Current = *d
			t.Nodename = data.Daemon.Nodename
			t.eventCount++
			t.app.QueueUpdateDraw(func() {
				// TODO: detect if t.updateInstanceView and t.updateConfigView need to be called (config mtime change, ...)
				switch t.focus() {
				case viewInstance:
					t.updateInstanceView()
				case viewConfig:
					t.updateConfigView()
				case viewKeys:
					t.updateKeysView()
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

func (t *App) updateObjects() {
	nodesCells := func(row int, selectable bool) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			t.objects.SetCell(row, t.firstInstanceCol+i, t.cellNode(nodename, selectable))
		}
	}

	nodesScoreCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeScore(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesLoadCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeLoad(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesMemCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeMem(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesSwapCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeSwap(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	nodesStateCells := func(row int) {
		for i, nodename := range t.Current.Cluster.Config.Nodes {
			s := tview.TranslateANSI(t.StrNodeStates(nodename))
			t.objects.SetCell(row, t.firstInstanceCol+i, tview.NewTableCell(s).SetSelectable(false))
		}
	}

	t.lastDraw = time.Now()

	t.objects.Clear()

	row := 0
	t.objects.SetCell(row, 0, tview.NewTableCell("CLUSTER").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell(t.Current.Cluster.Config.Name).SetSelectable(true))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("NODE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesCells(row, false)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("EVENT").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", t.eventCount)).SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("SCORE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesScoreCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("LAST").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("0s").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell(".LOAD").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesLoadCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell(".MEM").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesMemCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell(".SWAP").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesSwapCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("STATE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesStateCells(row)

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("").SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("┼").SetTextColor(colorTitle).SetSelectable(false))

	row++
	t.objects.SetCell(row, 0, tview.NewTableCell("PATH").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 1, tview.NewTableCell("AVAIL").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 2, tview.NewTableCell("ORCHESTRATE").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 3, tview.NewTableCell("UP").SetTextColor(colorTitle).SetSelectable(false))
	t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
	nodesCells(row, true)

	t.firstObjectRow = row + 1

	t.objects.SetFixed(t.firstObjectRow, 2)

	for _, path := range t.paths() {
		row++
		t.objects.SetCell(row, 0, t.cellObjectPath(path))
		t.objects.SetCell(row, 1, t.cellObjectStatus(path))
		t.objects.SetCell(row, 2, t.cellObjectOrchestrate(path))
		t.objects.SetCell(row, 3, t.cellObjectRunning(path))
		t.objects.SetCell(row, 4, tview.NewTableCell("│").SetTextColor(colorTitle).SetSelectable(false))
		for j, nodename := range t.Current.Cluster.Config.Nodes {
			t.objects.SetCell(row, 5+j, t.cellInstanceStatus(path, nodename))
		}
	}
}

func (t *App) cellObjectOrchestrate(path string) *tview.TableCell {
	s := t.Current.Cluster.Object[path].Orchestrate
	return tview.NewTableCell(s).SetSelectable(false)
}

func (t *App) cellObjectRunning(path string) *tview.TableCell {
	s := tview.TranslateANSI(t.StrObjectRunning(path))
	return tview.NewTableCell(s).SetSelectable(false)
}

func (t *App) cellObjectStatus(path string) *tview.TableCell {
	s := tview.TranslateANSI(monitor.StrObjectStatus(t.Current.Cluster.Object[path]))
	return tview.NewTableCell(s).SetSelectable(false)
}

func (t *App) cellInstanceStatus(path, node string) *tview.TableCell {
	s := tview.TranslateANSI(t.StrObjectInstance(path, node, t.Current.Cluster.Object[path].Scope))
	cell := tview.NewTableCell(s)
	if t.isInstanceSelected(path, node) {
		cell.SetBackgroundColor(colorSelected)
	}
	return cell
}

func (t *App) cellNode(node string, selectable bool) *tview.TableCell {
	cell := tview.NewTableCell(node).SetAttributes(tcell.AttrBold).SetSelectable(selectable)
	if selectable && t.isNodeSelected(node) {
		cell.SetBackgroundColor(colorSelected)
	}
	return cell
}

func (t *App) cellObjectPath(path string) *tview.TableCell {
	cell := tview.NewTableCell(path).SetAttributes(tcell.AttrBold)
	if t.isPathSelected(path) {
		cell.SetBackgroundColor(colorSelected)
	}
	return cell
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
		return true
	}
}

func (t *App) resetSelectedNodes() {
	if len(t.selectedNodes) == 0 {
		return
	}
	t.selectedNodes = make(map[string]any)
	for j := t.firstInstanceCol; j < t.objects.GetColumnCount(); j += 1 {
		t.objects.GetCell(t.firstObjectRow-1, j).SetBackgroundColor(colorNone)
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
		t.setFocus()
	}
	if t.command != nil {
		t.back()
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
			t.errorf("unknown command: %s", action)
		}
	}
	instanceAction := func(action string, keys map[[2]string]any) {
		switch action {
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
		case "switch":
			t.actionInstanceSwitch(keys)
		//	case "clear":
		//		t.actionInstanceClear(keys)
		default:
			t.errorf("unknown command: %s", action)
		}
	}
	nodeAction := func(action string, nodes map[string]any) {
		switch action {
		case "daemon restart":
			t.actionNodeDaemonRestart(nodes)
		case "freeze":
			t.actionNodeFreeze(nodes)
		case "unfreeze", "thaw":
			t.actionNodeUnfreeze(nodes)
		case "drain":
			t.actionNodeDrain(nodes)
		default:
			t.errorf("unknown command: %s", action)
		}
	}
	t.command = tview.NewInputField().
		SetLabel(":").
		SetFieldWidth(0).
		SetDoneFunc(func(key tcell.Key) {
			action := strings.TrimSpace(t.command.GetText())
			switch action {
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
			}
			switch key {
			case tcell.KeyEnter:
				switch {
				case len(t.selectedPaths) > 0:
					objectAction(action, t.selectedPaths)
				case len(t.selectedInstances) > 0:
					instanceAction(action, t.selectedInstances)
				case len(t.selectedNodes) > 0:
					nodeAction(action, t.selectedNodes)
				default:
					row, col := t.objects.GetSelection()
					switch {
					case row == 0 && col == 1:
						clusterAction(action)
					case row < t.firstObjectRow-1:
					case row == t.firstObjectRow-1:
						node := t.objects.GetCell(row, col).Text
						selection := make(map[string]any)
						selection[node] = nil
						nodeAction(action, selection)
					case col == 0:
						path := t.objects.GetCell(row, 0).Text
						selection := make(map[string]any)
						selection[path] = nil
						objectAction(action, selection)
					case col >= t.firstInstanceCol:
						path := t.objects.GetCell(row, 0).Text
						node := t.objects.GetCell(0, col).Text
						selection := make(map[[2]string]any)
						selection[[2]string{path, node}] = nil
						instanceAction(action, selection)
					}
				}
				clean()
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
	t.restart()
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
   Enter                Show instance status
   ESC                  Close popup

 Cluster commands:

   freeze, unfreeze

 Object commands:

   abort, delete, freeze, giveback, provision, purge, start, stop, switch,
   unfreeze, unprovision  

 Instance commands:

   delete, freeze, provision, start, stop, switch, unfreeze, unprovision  

 Node commands:

   drain freeze, unfreeze

`
	t.initTextView()
	t.textView.SetTitle("Help")
	fmt.Fprint(t.textView, help)
}

func (t *App) onRuneS(event *tcell.EventKey) {
}

func (t *App) onRuneL(event *tcell.EventKey) {
	row, col := t.objects.GetSelection()
	t.viewNode = ""
	t.viewPath = naming.Path{}
	if row >= t.firstObjectRow {
		path := t.objects.GetCell(row, 0).Text
		p, err := naming.ParsePath(path)
		if err != nil {
			return
		}
		t.viewPath = p
	}
	if col >= t.firstInstanceCol {
		t.viewNode = t.objects.GetCell(t.firstObjectRow-1, col).Text
	}

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

	t.initTextView()
	t.nav(viewLog)
	t.textView.SetDynamicColors(true)
	t.textView.SetTitle(title())
	t.textView.SetChangedFunc(func() {
		t.textView.ScrollToEnd()
	})

	t.textView.Clear()

	lines := 50
	follow := true
	log := t.client.NewGetLogs(t.viewNode).
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

func (t *App) onEnter(event *tcell.EventKey) {
	row, col := t.objects.GetSelection()
	t.viewPath = naming.Path{}
	t.viewNode = ""
	if row >= t.firstObjectRow {
		path := t.objects.GetCell(row, 0).Text
		p, err := naming.ParsePath(path)
		if err != nil {
			return
		}
		t.viewPath = p
	}
	if col >= t.firstInstanceCol {
		node := t.objects.GetCell(t.firstObjectRow-1, col).Text
		t.viewNode = node
	}
	switch {
	case !t.viewPath.IsZero() && t.viewNode != "":
		t.initTextView()
		t.nav(viewInstance)
	case t.viewPath.Kind == naming.KindCfg || t.viewPath.Kind == naming.KindSec:
		t.nav(viewKeys)
	}
}

func (t *App) setFocus() {
	switch t.focus() {
	case viewConfig:
		t.app.SetFocus(t.textView)
	case viewInstance:
		t.app.SetFocus(t.textView)
	case viewLog:
		t.app.SetFocus(t.textView)
	case viewKeys:
		t.app.SetFocus(t.keys)
	default:
		t.app.SetFocus(t.objects)
	}
}

func (t *App) updateKeysView() {
	if t.viewPath.IsZero() {
		return
	}
	t.initKeysTable()
	resp, err := t.client.GetObjectKVStoreKeysWithResponse(context.Background(), t.viewPath.Namespace, t.viewPath.Kind, t.viewPath.Name)
	if err != nil {
		return
	}
	if resp.StatusCode() != http.StatusOK {
		return
	}
	t.keys.Clear()
	t.keys.SetCell(0, 0, tview.NewTableCell("NAME").SetTextColor(colorTitle))
	t.keys.SetCell(0, 1, tview.NewTableCell("SIZE").SetTextColor(colorTitle))
	for i, key := range resp.JSON200.Items {
		row := 1 + i
		t.keys.SetCell(row, 0, tview.NewTableCell(key.Key).SetSelectable(true))
		t.keys.SetCell(row, 1, tview.NewTableCell(sizeconv.BSizeCompact(float64(key.Size))))
	}
}

func (t *App) updateInstanceView() {
	digest := t.Frame.Current.GetObjectStatus(t.viewPath)
	text := tview.TranslateANSI(digest.Render([]string{t.viewNode}))
	t.initTextView()
	title := fmt.Sprintf("%s@%s status", t.viewPath, t.viewNode)
	t.textView.SetDynamicColors(true)
	t.textView.SetTitle(title)
	t.textView.Clear()
	fmt.Fprint(t.textView, text)
}

func (t *App) onRuneC(event *tcell.EventKey) {
	row, _ := t.objects.GetSelection()
	path := t.objects.GetCell(row, 0).Text
	p, err := naming.ParsePath(path)
	if err != nil {
		return
	}
	t.viewPath = p
	t.viewNode = ""
	t.initTextView()
	t.updateConfigView()
	t.nav(viewConfig)
}

func (t *App) updateConfigView() {
	if t.viewPath.IsZero() {
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
	title := fmt.Sprintf("%s configuration", t.viewPath)
	t.textView.SetDynamicColors(false)
	t.textView.SetTitle(title)
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
	if to == from {
		return
	}
	t.navFromTo(from, to)
}

func (t *App) navFromTo(from, to viewId) {
	t.flex.Clear()
	switch from {
	case viewObject:
	case viewLog:
		t.textView.SetChangedFunc(nil)
		t.textView = nil
		t.logCloser.Close()
	case viewConfig, viewInstance, viewKey:
		t.textView = nil
	case viewKeys:
		t.keys = nil
	}
	switch to {
	case viewLog, viewConfig, viewKey:
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
	case viewInstance:
		t.flex.AddItem(t.textView, 0, 1, true)
		t.app.SetFocus(t.textView)
		t.updateInstanceView()
	case viewKeys:
		t.updateKeysView()
		t.flex.AddItem(t.keys, 0, 1, true)
		t.app.SetFocus(t.keys)
	case viewObject:
		t.flex.AddItem(t.objects, 0, 1, true)
		t.app.SetFocus(t.objects)
		t.updateObjects()
	}
	t.flex.AddItem(t.errs, 1, 0, false)
}

func (t *App) initTextView() {
	if t.textView != nil {
		return
	}
	v := tview.NewTextView()
	v.SetScrollable(true)
	v.SetBorder(true)
	v.SetBorderPadding(1, 1, 1, 1)
	v.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyESC:
			t.back()
		}
		switch event.Rune() {
		case 'q':
			t.stop()
		case ':':
			t.onRuneColumn(event)
		default:
			return event
		}
		return nil
	})

	t.textView = v
	return
}

func (t *App) restart() {
	t.restartC <- nil
}

func (t *App) stop() {
	t.exitFlag.Store(true)
	t.errC <- nil
	t.app.Stop()
}
