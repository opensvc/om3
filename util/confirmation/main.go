//go:build !solaris

package confirmation

import (
	"fmt"
	"time"

	"github.com/atomicgo/cursor"
	"github.com/eiannone/keyboard"
)

func ReadLn(description string, timeout time.Duration) (string, error) {
	if len(description) > 0 {
		fmt.Println(description)
		fmt.Println("")
	}
	keysEvents, err := keyboard.GetKeys(10)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = keyboard.Close()
	}()

	area := cursor.NewArea()
	end := time.Now().Add(timeout)
	word := ""
	pos := 0

	prompt := func() bool {
		left := end.Sub(time.Now())
		if left < 0 {
			return false
		}
		area.Clear()
		s := fmt.Sprintf("Timeout in %s > %s", left.Round(time.Second), word)
		area.Update(s)
		cursor.Up(1)
		cursor.HorizontalAbsolute(len(s) - pos)
		return true
	}

	_ = prompt()

	for {
		select {
		case ev := <-keysEvents:
			switch {
			case ev.Err != nil:
				fmt.Println("")
				return "", err
			case (ev.Key == keyboard.KeyBackspace) || (ev.Key == keyboard.KeyBackspace2):
				if len(word) > 0 {
					word = word[0 : len(word)-1]
				}
				_ = prompt()
			case ev.Key == keyboard.KeyCtrlC:
				fmt.Println("")
				return "", fmt.Errorf("interrupted")
			case ev.Key == keyboard.KeyEnter:
				fmt.Println("")
				return word, nil
			case ev.Key == keyboard.KeyArrowLeft:
				if pos < len(word) {
					pos = pos + 1
					_ = prompt()
				}
			case ev.Key == keyboard.KeyArrowRight:
				if pos > 0 {
					pos = pos - 1
					_ = prompt()
				}
			case ev.Key == keyboard.KeyCtrlE:
				if pos > 0 {
					pos = 0
					_ = prompt()
				}
			case ev.Key == keyboard.KeyCtrlA:
				if pos < len(word) {
					pos = len(word)
					_ = prompt()
				}
			case ev.Key == keyboard.KeyCtrlK:
				if pos > 0 {
					word = word[0 : len(word)-pos]
					pos = 0
					_ = prompt()
				}
			case ev.Key == keyboard.KeySpace:
				offset := len(word) - pos
				word = word[0:offset] + " " + word[offset:]
				_ = prompt()
			case ev.Rune != '0':
				offset := len(word) - pos
				word = word[0:offset] + string(ev.Rune) + word[offset:]
				_ = prompt()
			}
		case <-time.After(time.Second):
			if more := prompt(); !more {
				fmt.Println("")
				return "", fmt.Errorf("timeout")
			}
		}
	}
	fmt.Println("")
	return "", nil
}
