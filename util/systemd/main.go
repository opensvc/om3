//go:build linux

package systemd

import (
	"fmt"
	"os"
	"strings"
)

// HasSystemd return true if systemd is detected on current os
func HasSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	return true
}

func PGID2Slice(s string) string {
	slice := ""
	for _, e := range strings.Split(s, ".slice") {
		slice += Escape(e)
	}
	return slice + ".slice"
}

func Escape(s string) string {
	var result strings.Builder

	for _, r := range s {
		switch r {
		case '/':
			result.WriteString("-")
		case '-', '_':
			result.WriteRune(r)
			//		case '.', '\\', '\n', '\r', '\t':
			// Escape special characters with C-style backslash escaping
			//			result.WriteString(fmt.Sprintf("\\x%02x", r))
		default:
			// Check if the character is printable ASCII
			if r >= 32 && r <= 126 {
				result.WriteRune(r)
			} else {
				// Escape non-printable ASCII characters
				result.WriteString(fmt.Sprintf("\\x%02x", r))
			}
		}
	}

	return result.String()
}
