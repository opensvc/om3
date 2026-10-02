package oidc

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
)

// ErrNoBrowser is returned when no browser can be opened on this machine.
var ErrNoBrowser = errors.New("no browser to open")

// OpenBrowser opens the browser of the user on url.
//
// On linux and the other unixes, a browser needs a graphical session: one
// reached through ssh has none, and its user is asked to log in with a device
// code instead.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		if !HasBrowser() {
			return ErrNoBrowser
		}
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return errors.Join(ErrNoBrowser, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// HasBrowser says whether a browser can be opened on this machine.
func HasBrowser() bool {
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	}
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	_, err := exec.LookPath("xdg-open")
	return err == nil
}
