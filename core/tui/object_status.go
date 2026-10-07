package tui

import (
	"github.com/rivo/tview"

	"github.com/opensvc/om3/v3/core/statusboard"
)

// updateObjectStatusView shows the status board of the object of the path
// selected, as "om <path> status" does: the resources by node, the instance
// states, and the notes saying what needs attention.
func (t *App) updateObjectStatusView() {
	if t.viewPath.IsZero() || t.textView == nil {
		return
	}
	digest := t.Current.GetObjectStatus(t.viewPath)
	// The text view is not laid out yet when the view is entered: the
	// width is the one of the screen layout, which is.
	_, _, width, _ := t.flex.GetInnerRect()
	text := statusboard.Render(digest, width-1)
	t.textView.SetDynamicColors(true)
	t.textView.SetTitle(t.viewPath.String() + " status")
	t.textView.SetText(tview.TranslateANSI(tview.Escape(text)))
}
