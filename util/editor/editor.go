package editor

import (
	"os"
	"os/exec"
	"runtime"
)

func Edit(p string) error {
	name := os.Getenv("EDITOR")
	if name == "" {
		switch runtime.GOOS {
		case "windows":
			name = "notepad.exe"
		default:
			name = "vi"
		}
	}
	cmd := exec.Command(name, p)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
