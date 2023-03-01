package progress

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/atomicgo/cursor"
	"golang.org/x/term"
)

type (
	View struct {
		keys      []string
		lines     map[string]viewLine
		cmdQ      chan any
		renderQ   chan any
		format    string
		keyWidth  int
		width     int
		sep       string
		displayed int
	}
	viewLine struct {
		key string
		msg string
	}

	msgExit struct{}
)

const contextKey = 0

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

func NewView() *View {
	v := View{
		keys:  make([]string, 0),
		lines: make(map[string]viewLine),
		cmdQ:  make(chan any),
		sep:   " │ ",
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
				case viewLine:
					if _, ok := v.lines[m.key]; !ok {
						v.keys = append(v.keys, m.key)
					}
					v.lines[m.key] = m
					v.updateLineWidths(m)
					v.render()
				}
			case <-ticker.C:
				v.updateTermWidth()
				v.render()
			}
		}
	}()
}

func (v *View) updateLineWidths(line viewLine) {
	n := len(line.key)
	if n > v.keyWidth {
		v.keyWidth = n
	}
	v.format = "%-" + fmt.Sprint(v.keyWidth) + "s" + v.sep + "%s\n"
}

func (v *View) updateTermWidth() {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err != nil {
		v.width = 80
	} else {
		v.width = w
	}
}

func (v *View) Info(key, msg string) {
	line := viewLine{
		key: key,
		msg: msg,
	}
	v.cmdQ <- line

}

func (v *View) Infof(key, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	v.Info(key, msg)
}

func (v *View) render() {
	// prevent erasing above the cursor on first render
	toDisplay := len(v.keys)
	if v.displayed > 0 {
		cursor.StartOfLineUp(v.displayed)
	}
	v.displayed = toDisplay

	for _, key := range v.keys {
		line := v.lines[key]
		cursor.ClearLine()
		fmt.Printf(v.format, line.key, line.msg)
	}
}
