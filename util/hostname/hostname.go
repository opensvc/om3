package hostname

import (
	"fmt"
	"os"
	"strings"
)

const (
	alnums = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var (
	hostname string
)

func IsValid(s string) bool {
	if err := validate(s); err != nil {
		return false
	}
	return true
}

func validate(s string) error {
	n := len(s)
	switch {
	case n < 1:
		return fmt.Errorf("too short (<1)")
	case n > 63:
		return fmt.Errorf("too long (>63)")
	}
	if strings.Trim(s[0:1], alnums) != "" {
		return fmt.Errorf("invalid first character")
	}
	if strings.Trim(s[1:], alnums+"-") != "" {
		return fmt.Errorf("invalid characters")
	}
	return nil
}

// StrictHostname is like os.StrictHostname except it returns a lowercased hostname,
// and it caches the result to avoid repeating syscalls
func StrictHostname() (string, error) {
	if hostname != "" {
		return hostname, nil
	}
	if s, err := os.Hostname(); err == nil {
		hostname = strings.ToLower(s)
		return hostname, nil
	} else {
		return "", err
	}
}

func Hostname() string {
	h, _ := StrictHostname()
	return h
}

// OtherNodes returns list of nodes without local hostname
func OtherNodes(nodes []string) []string {
	var oNodes []string
	for _, node := range nodes {
		if node == Hostname() {
			continue
		}
		oNodes = append(oNodes, node)
	}
	return oNodes
}

func Error() error {
	if _, err := StrictHostname(); err != nil {
		return err
	}
	return nil
}

// Impersonate eases testing
func Impersonate(s string) func() {
	saved := "" + hostname
	fn := func() { hostname = saved }
	hostname = s
	return fn
}

// SetHostnameForGoTest can be used during go test to define alternate hostname
//
// Example:
//
//	func Test_something(t *testing.T) {
//	  SetHostnameForGoTest("newhostname")
//	  defer SetHostnameForGoTest("")
//	  // test...
//	}
func SetHostnameForGoTest(s string) {
	hostname = s
}
