package xspin

import (
	"fmt"
	"strings"
	"sync"

	"github.com/atomicgo/cursor"
)

type (
	Spinner struct {
		frames           []string
		index            int
		disabled         bool
		msg              string
		writtenRuneCount int
		mu               sync.Mutex
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
	s.mu.Lock()
	s.erase()
	s.mu.Unlock()
}

func (s Spinner) erase() {
	frame := strings.Repeat(" ", s.writtenRuneCount)
	cursor.Left(s.writtenRuneCount)
	fmt.Print(frame)
	cursor.Left(s.writtenRuneCount)
}

func (s *Spinner) Draw() {
	s.mu.Lock()
	s.draw()
	s.mu.Unlock()
}

func (s *Spinner) draw() {
	frame := s.String()
	s.writtenRuneCount = len([]rune(frame))
	fmt.Print(frame)
}

func (s *Spinner) Redraw() {
	s.mu.Lock()
	if s.disabled {
		return
	}
	s.erase()
	s.draw()
	s.mu.Unlock()
}

func (s Spinner) String() string {
	return fmt.Sprint(s.frames[s.index] + " " + s.msg)
}

func (s *Spinner) Enable() {
	s.mu.Lock()
	s.disabled = false
	s.mu.Unlock()
}

func (s *Spinner) Disable() {
	s.mu.Lock()
	s.disabled = true
	s.mu.Unlock()
}

func (s *Spinner) Tick(msg string) {
	s.mu.Lock()
	s.msg = msg
	if s.index >= (len(s.frames) - 1) {
		s.index = 0
	} else {
		s.index += 1
	}
	s.mu.Unlock()
}
