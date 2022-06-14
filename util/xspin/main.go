package xspin

import (
	"fmt"
	"strings"

	"github.com/atomicgo/cursor"
)

type (
	Spinner struct {
		frames           []string
		index            int
		disabled         bool
		msg              string
		writtenRuneCount int
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
	frame := strings.Repeat(" ", s.writtenRuneCount)
	cursor.Left(s.writtenRuneCount)
	fmt.Print(frame)
	cursor.Left(s.writtenRuneCount)
}

func (s *Spinner) Draw() {
	frame := s.String()
	s.writtenRuneCount = len([]rune(frame))
	fmt.Print(frame)
}

func (s *Spinner) Redraw() {
	if s.disabled {
		return
	}
	s.Erase()
	s.Draw()
}

func (s Spinner) String() string {
	return fmt.Sprint(s.frames[s.index] + " " + s.msg)
}

func (s *Spinner) Enable() {
	s.disabled = false
}

func (s *Spinner) Disable() {
	s.disabled = true
}

func (s *Spinner) Tick(msg string) {
	s.msg = msg
	if s.index >= (len(s.frames) - 1) {
		s.index = 0
	} else {
		s.index += 1
	}
}
