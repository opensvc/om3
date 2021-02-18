package render

import (
	"fmt"
	"strings"
)

// Listener formats a listener string, takinf care of bracketing IPV6 addresses
func Listener(addr string, port int64) string {
	if strings.Contains(addr, ":") {
		return fmt.Sprintf("[%s]:%d", addr, port)
	}
	return fmt.Sprintf("%s:%d", addr, port)
}
