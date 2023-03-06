package progress

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/atomicgo/cursor"
	"golang.org/x/term"
)

type (
	View struct {
		nodes     *nodes
		cmdQ      chan any
		renderQ   chan any
		format    string
		widths    []int
		width     int
		displayed int
	}
	nodes struct {
		l []*node
	}
	node struct {
		key   string
		msg   []string
		nodes *nodes
	}
	info struct {
		keys []string
		msg  []*string
	}

	msgExit struct{}
)

const (
	contextKey = 0
	ansi       = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
)

var (
	PadFiller   = "  "
	PadNextNode = "├ "
	PadLastNode = "└ "

	re = regexp.MustCompile(ansi)
)

func realLen(s string) int {
	return len(stripAnsi(s))
}

func stripAnsi(s string) string {
	return re.ReplaceAllString(s, "")
}

func ContextWithNewView(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey, NewView())
}

func ContextWithView(ctx context.Context, p *View) context.Context {
	return context.WithValue(ctx, contextKey, p)
}

func ViewFromContext(ctx context.Context) *View {
	if i := ctx.Value(contextKey); i != nil {
		return i.(*View)
	}
	return nil
}

func newNodes() *nodes {
	t := nodes{
		l: make([]*node, 0),
	}
	return &t
}

func NewView() *View {
	v := View{
		nodes: newNodes(),
		cmdQ:  make(chan any),
	}
	return &v
}

func (v *View) Stop() {
	v.cmdQ <- msgExit{}
}

func (v *View) Start() {
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case i := <-v.cmdQ:
				switch m := i.(type) {
				case msgExit:
					return
				case info:
					v.mergeInfo(m)
				}
			case <-ticker.C:
				v.updateTermWidth()
				v.render()
			}
		}
	}()
}

func msgWidths(msg []*string) []int {
	widths := make([]int, len(msg))
	for i, s := range msg {
		if s == nil {
			continue
		}
		widths[i] = realLen(*s)
	}
	return widths
}

func (t info) widths() (widths []int) {
	keyWidth := 0
	for i, s := range t.keys {
		w := len(s) + i*len(PadFiller)
		if w > keyWidth {
			keyWidth = w
		}
	}
	widths = append(widths, keyWidth)
	widths = append(widths, msgWidths(t.msg)...)
	return
}

func (v *View) mergeInfo(m info) {
	v.nodes.merge(m)
	v.updateWidths(m)
}

func (v *View) updateWidths(m info) {
	widths := m.widths()
	n := len(v.widths)
	for i, w := range widths {
		if i >= n-1 {
			v.widths = append(v.widths, w)
		} else if w > v.widths[i] {
			v.widths[i] = w
		}
	}
	v.render()
}

func (t *nodes) merge(m info) {
	switch len(m.keys) {
	case 0:
		// refuse to merge a msg with empty keys
	default:
		key := m.keys[0]
		m.keys = m.keys[1:]
		t.get(key).merge(m)
	}
	return
}

func (t nodes) getStrict(key string) *node {
	for _, n := range t.l {
		if n.key == key {
			return n
		}
	}
	return nil
}

func (t *nodes) get(key string) *node {
	if ptr := t.getStrict(key); ptr != nil {
		return ptr
	}
	n := node{
		key:   key,
		nodes: newNodes(),
	}
	t.l = append(t.l, &n)
	return &n
}

func (t nodes) len() int {
	return len(t.l)
}

func (t nodes) list() []*node {
	return t.l
}

func (t *node) merge(m info) {
	switch len(m.keys) {
	case 0:
		curColCount := len(t.msg)
		for i, v := range m.msg {
			if v == nil {
				continue
			}
			if i >= curColCount {
				t.msg = append(t.msg, *v)
			} else {
				t.msg[i] = *v
			}
		}
		return
	default:
		key := m.keys[0]
		m.keys = m.keys[1:]
		t.nodes.get(key).merge(m)
		return
	}
}

func (v *View) updateTermWidth() {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err != nil {
		v.width = 80
	} else {
		v.width = w
	}
}

func (v *View) Info(key []string, msg []*string) {
	line := info{
		keys: key,
		msg:  msg,
	}
	v.cmdQ <- line

}

func (t node) len() int {
	i := t.nodes.len()
	for _, node := range t.nodes.list() {
		i += node.len()
	}
	return i
}

func (v *View) len() int {
	i := v.nodes.len()
	for _, node := range v.nodes.list() {
		i += node.len()
	}
	return i
}

func (v *View) render() {
	// prevent erasing above the cursor on first render
	toDisplay := v.len()
	if v.displayed > 0 {
		cursor.StartOfLineUp(v.displayed)
	}
	v.displayed = toDisplay
	v.nodes.render(v.widths, 0)
}

func (t nodes) render(widths []int, depth int) {
	max := len(t.l) - 1
	for i, n := range t.l {
		n.render(widths, depth, i == max)
	}
}

func leftPadPrint(width int, s string) {
	w := width + len(s) - realLen(s)
	fmt.Printf("%-"+fmt.Sprint(w)+"s", s)
}

func (t node) render(widths []int, depth int, last bool) {
	cursor.ClearLine()
	padding := ""
	if depth > 0 {
		for i := 0; i < depth-1; i += 1 {
			padding += PadFiller
		}
		if last {
			padding += PadLastNode
		} else {
			padding += PadNextNode
		}
	}
	fmt.Print(padding)
	leftPadPrint(widths[0]-len(padding)+1, t.key)
	for i, s := range t.msg {
		leftPadPrint(widths[i+1]+1, s)
	}
	fmt.Println("")
	t.nodes.render(widths, depth+1)
}
