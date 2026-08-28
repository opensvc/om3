package tui

import (
	"strings"
	"time"

	"github.com/rivo/tview"
)

type (
	// viewId identifies a view. Every view must have an entry in viewDefs.
	viewId int

	// viewDef describes the lifecycle of a view. It is the single place where
	// a view declares its name, how it puts itself on screen, how it keeps
	// itself up to date and what it has to release when the user leaves it.
	viewDef struct {
		// title names the view in the head bar and in the navigation stack.
		title string

		// enter builds the view primitive, mounts it and populates it. It is
		// called when the view gets the focus, either by navigating forward
		// with nav() or backward with back().
		enter func(*App)

		// refresh repopulates the view on a cluster data update. Left nil by
		// the views that don't depend on the cluster data, or that stream
		// their own content.
		refresh func(*App)

		// leave releases what enter and refresh acquired: readers, contexts
		// and cached primitives. Left nil by the views owning no resource.
		leave func(*App)
	}

	// frame is a navigation stack entry: a view plus the state the user
	// expects to find again when navigating back to it.
	frame struct {
		id viewId

		// position is the cursor of the view table.
		position Position

		// selectedElement is the element the view drilled down into, ie the
		// pool name of a pool volumes view. Inherited from the parent frame
		// on push, restored on pop.
		selectedElement string
	}

	viewStack []frame

	// mountBanner is a fixed height primitive displayed between the head bar
	// and the view body.
	mountBanner struct {
		primitive tview.Primitive
		height    int
	}

	// mountSpec is the layout of a mounted view, remembered so the layout can
	// be restored after a full screen popup like the help.
	mountSpec struct {
		body    tview.Primitive
		banners []mountBanner
	}
)

const (
	viewObject viewId = iota
	viewContext
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
	viewHbStatus
	viewRelay
)

// viewDefs is the view registry. Adding a view is adding an entry here.
//
// The views whose enter and refresh go through createTable() mount themselves:
// createTable() calls mount() when it has to replace the displayed table.
var viewDefs map[viewId]viewDef

func init() {
	viewDefs = map[viewId]viewDef{
		viewObject: {
			title: "objects",
			enter: func(t *App) {
				t.mount(t.objects)
				t.updateObjects()
			},
			refresh: (*App).updateObjects,
		},
		viewContext: {
			title: "context",
			enter: (*App).updateContextList,
		},
		viewConfig: {
			title: "configuration",
			enter: func(t *App) {
				t.mountTextView()
				t.updateConfigView()
			},
			refresh: (*App).updateConfigView,
			leave:   (*App).releaseTextView,
		},
		viewKey: {
			title: "key",
			enter: func(t *App) {
				t.mountTextView()
				t.updateKeyTextView()
			},
			leave: (*App).releaseTextView,
		},
		viewKeys: {
			title: "keys",
			enter: func(t *App) {
				t.initKeysTable()
				t.mount(t.keys)
				t.updateKeysView()
			},
			refresh: (*App).updateKeysView,
			leave:   func(t *App) { t.keys = nil },
		},
		viewInstance: {
			title: "instance",
			enter: func(t *App) {
				// updateInstanceView() mounts its own primitives, but it bails
				// out when the instance data is not there yet: mount a body so
				// the screen is never left without one.
				t.mountTextView()
				t.updateInstanceView()
			},
			refresh: (*App).updateInstanceView,
			leave:   (*App).releaseTextView,
		},
		viewLog: {
			title: "log",
			enter: func(t *App) {
				t.mountTextView()
				t.logCloser.Reset()
				// updateLogTextView() opens the log readers and lets them stream
				// into the text view: it must not be called on data updates.
				t.updateLogTextView()
			},
			leave: func(t *App) {
				// Don't clear the changed handler: TextView.SetChangedFunc()
				// writes an unlocked field, which the log readers are reading
				// from their own goroutine. The handler only asks for a
				// redraw, so it is harmless on a text view left behind.
				t.releaseTextView()
				t.logCloser.CloseAll()
			},
		},
		viewEvents: {
			title: "events",
			enter: func(t *App) {
				t.isInEventView.Store(true)
				t.mountTextView()
				t.initEventsView()
				t.updateEventsView()
			},
			refresh: (*App).updateEventsView,
			leave: func(t *App) {
				t.isInEventView.Store(false)
				if t.eventsCancel != nil {
					t.eventsCancel()
				}
				t.releaseTextView()
			},
		},
		viewPool: {
			title:   "pool",
			enter:   (*App).updatePools,
			refresh: (*App).updatePools,
		},
		viewPoolVolume: {
			title:   "pool volume",
			enter:   (*App).updatePoolVolumes,
			refresh: (*App).updatePoolVolumes,
		},
		viewNetwork: {
			title:   "network",
			enter:   (*App).updateNetworkList,
			refresh: (*App).updateNetworkList,
		},
		viewNetworkIpList: {
			title:   "network ip list",
			enter:   (*App).updateNetworkIps,
			refresh: (*App).updateNetworkIps,
		},
		viewHbStatus: {
			title:   "heartbeat status",
			enter:   (*App).updateHbStatus,
			refresh: (*App).updateHbStatus,
		},
		viewRelay: {
			title:   "relay",
			enter:   (*App).updateRelayStatus,
			refresh: (*App).updateRelayStatus,
		},
	}
}

func (t viewId) String() string {
	return viewDefs[t].title
}

func (t viewStack) String() string {
	l := make([]string, len(t))
	for i, f := range t {
		l[i] = f.id.String()
	}
	return strings.Join(l, " > ")
}

//
// Navigation stack
//

// frame returns the focused stack frame. The stack is never empty: its first
// frame is the root view, the one ESC can not pop.
func (t *App) frame() *frame {
	return &t.stack[len(t.stack)-1]
}

// focus returns the id of the focused view. The navigation stack is owned by
// the tview loop: call focusAsync() from any other goroutine.
func (t *App) focus() viewId {
	return t.frame().id
}

// focusAsync returns the id of the focused view, read from the atomic mirror
// enterView() keeps up to date. Safe to call off the tview loop.
func (t *App) focusAsync() viewId {
	return viewId(t.focusedView.Load())
}

// atRoot returns true when the focused view is the one ESC can not pop.
func (t *App) atRoot() bool {
	return len(t.stack) == 1
}

// resetStack makes v the only frame of the navigation stack, without entering
// it.
func (t *App) resetStack(v viewId) {
	t.stack = viewStack{{id: v}}
}

// nav pushes a new frame on the navigation stack and enters it.
//
// Navigating to the frame already on top is a no-op: a view is identified by
// its id and by the element it drilled down into, so that entering a pool from
// the pool list does push a frame, while hitting the log key twice does not.
func (t *App) nav(to viewId) {
	if f := t.frame(); f.id == to && f.selectedElement == t.selectedElement {
		return
	}
	t.leaveView(t.focus())
	t.stack = append(t.stack, frame{id: to, selectedElement: t.selectedElement})
	t.enterView(to)
}

// navRoot enters v and makes it the root of a new navigation stack.
func (t *App) navRoot(v viewId) {
	t.leaveView(t.focus())
	t.resetStack(v)
	t.enterView(v)
}

// back pops the focused frame and re-enters the one below, restoring the
// element it had drilled down into.
//
// At the root of the stack there is nothing to pop: the object view then
// resets its selector to the one asked on the command line.
func (t *App) back() {
	if t.atRoot() {
		if t.focus() == viewObject {
			t.setFilter(t.defaultSelector())
		}
		return
	}
	t.leaveView(t.focus())
	t.stack = t.stack[:len(t.stack)-1]
	t.selectedElement = t.frame().selectedElement
	t.enterView(t.focus())
}

func (t *App) enterView(v viewId) {
	t.focusedView.Store(int32(v))
	t.lastUpdatedAt = time.Time{}
	if enter := viewDefs[v].enter; enter != nil {
		enter(t)
	}
	// enter() sets the view title after mounting: refresh the head bar so it
	// names the view without waiting for the next cluster data update.
	t.updateHead()
}

func (t *App) leaveView(v viewId) {
	if leave := viewDefs[v].leave; leave != nil {
		leave(t)
	}
}

// refreshView repopulates the focused view. Called on every cluster data
// update.
func (t *App) refreshView() {
	if t.help != nil {
		// don't pull the help popup from under the reader
		return
	}
	if refresh := viewDefs[t.focus()].refresh; refresh != nil {
		refresh(t)
	}
	t.updateHead()
}

// defaultSelector returns the object selector the object view falls back to.
func (t *App) defaultSelector() string {
	if t.options != nil && t.options.Selector != "" {
		return t.options.Selector
	}
	return "*/svc/*"
}

//
// Layout
//

// mount lays the application out: the head bar on top, the optional fixed
// height banners, the view body taking the remaining height and the focus,
// and the errors bar at the bottom.
func (t *App) mount(body tview.Primitive, banners ...mountBanner) {
	t.remount(mountSpec{body: body, banners: banners})
}

func (t *App) remount(spec mountSpec) {
	t.mounted = spec
	t.flex.Clear()
	t.flex.AddItem(t.head, 1, 0, false)
	for _, banner := range spec.banners {
		t.flex.AddItem(banner.primitive, banner.height, 0, false)
	}
	t.flex.AddItem(spec.body, 0, 1, true)
	t.flex.AddItem(t.errs, 1, 0, false)
	t.app.SetFocus(spec.body)
	t.updateHead()
}

// body returns the mounted view body, the primitive to give the focus back to
// after a popup.
func (t *App) body() tview.Primitive {
	return t.mounted.body
}

// mountTextView mounts the shared text view, creating it if needed.
func (t *App) mountTextView() {
	if t.textView == nil {
		v := tview.NewTextView()
		v.SetScrollable(true)
		v.SetBorder(false)
		t.textView = v
	}
	t.mount(t.textView)
}

func (t *App) releaseTextView() {
	t.textView = nil
}
