package xspin

import (
	"fmt"
	"strings"

	"github.com/atomicgo/cursor"
)

type (
	Spinner struct {
		frames   []string
		index    int
		disabled bool
	}
)

var (
	patterns = map[string][]string{
		"circle": []string{"◜ ", " ◝", " ◞", "◟ "},
		"slash":  []string{"|", "/", "-", "\\"},
		"wave":   []string{"▁", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▁"},
	}
)

func New(kind string) *Spinner {
	frames, ok := patterns[kind]
	if !ok {
		frames = patterns["slash"]
	}
	return &Spinner{
		frames: frames,
	}
}

func (s Spinner) Erase() {
	frame := s.String()
	n := len([]rune(frame))
	frame = strings.Repeat(" ", n)
	cursor.Left(n)
	fmt.Print(frame)
}

func (s Spinner) Draw() {
	fmt.Print(s)
}

func (s Spinner) Redraw() {
	if s.disabled {
		return
	}
	frame := s.String()
	cursor.Left(len([]rune(frame)))
	fmt.Print(frame)
}

func (s Spinner) String() string {
	return fmt.Sprint(s.frames[s.index] + " ")
}

func (s *Spinner) Enable() {
	s.disabled = false
}

func (s *Spinner) Disable() {
	s.disabled = true
}

func (s *Spinner) Tick() {
	if s.index >= (len(s.frames) - 1) {
		s.index = 0
	} else {
		s.index += 1
	}
}
