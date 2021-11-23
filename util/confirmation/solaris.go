// +build solaris

package confirmation

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

func ReadLn(description string, timeout time.Duration) (string, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return "", err
	}
	if err := s.Init(); err != nil {
		return "", err
	}
	defer func() {
		s.Fini()
	}()
	s.DisableMouse()
	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	end := time.Now().Add(timeout)
	word := ""
	pos := 0

	descriptionLines := 0
	for i, line := range strings.Split(description, "\n") {
		s.SetContent(0, i, rune(0), []rune(line), defStyle)
		descriptionLines = descriptionLines + 1
	}
	prompt := func() bool {
		left := end.Sub(time.Now())
		if left < 0 {
			return false
		}
		var data []rune
		ss := fmt.Sprintf("Timeout in %s > %s", left.Round(time.Second), word)
		data = []rune(ss)
		col, _ := s.Size()
		if col == 0 {
			col = 80
		}
		lines := splitCol(data, col)
		for i := 0; i < len(lines); i++ {
			s.SetContent(0, i+descriptionLines, rune(0), lines[i], defStyle)
		}

		s.ShowCursor((len(data)-pos+1)%col, descriptionLines+(len(data)-pos+1)/col)
		s.Sync()
		return true
	}

	getKeys := func(bufferSize int) (<-chan *tcell.EventKey, error) {
		inputComm := make(chan *tcell.EventKey, bufferSize)
		go func() {
			for {
				ev := s.PollEvent()
				switch eventKey := ev.(type) {
				case *tcell.EventKey:
					inputComm <- eventKey
				}
			}
		}()
		return inputComm, nil
	}

	_ = prompt()

	inputComm, err := getKeys(1)

	for {
		select {
		case ev := <-inputComm:
			key := ev.Key()
			switch {
			case (key == tcell.KeyEscape || key == tcell.KeyCtrlC):
				fmt.Println("")
				return "", fmt.Errorf("interrupted")
			case (key == tcell.KeyBackspace) || (key == tcell.KeyBackspace2):
				offset := len(word) - pos
				if offset > 0 {
					word = word[0:offset-1] + word[offset:len(word)]
					_ = prompt()
				}
			case key == tcell.KeyLeft:
				if pos < len(word) {
					pos = pos + 1
					_ = prompt()
				}
			case key == tcell.KeyRight:
				if pos > 0 {
					pos = pos - 1
					_ = prompt()
				}
			case key == tcell.KeyEnter:
				fmt.Println("")
				return string(word), nil
			default:
				offset := len(word) - pos
				word = word[0:offset] + string(ev.Rune()) + word[offset:len(word)]
				_ = prompt()
			}
		case <-time.After(time.Second):
			if more := prompt(); !more {
				fmt.Println("")
				return "", fmt.Errorf("timeout")
			}
		}
	}
}

func splitCol(data []rune, col int) (result [][]rune) {
	var newLine []rune
	for i := 0; i <= len(data)/col; i = i + 1 {
		if len(data[i*col:]) >= col {
			newLine = data[i*col : (i+1)*col]
		} else {
			newLine = data[i*col:]
		}
		if len(newLine) > 0 {
			result = append(result, newLine)
		}
	}
	return result
}
