package hostname

import (
	"os"
	"regexp"
	"strings"
)

const regexStringRFC952 = `^[a-zA-Z]([a-zA-Z0-9\-]+[\.]?)*[a-zA-Z0-9]$` // https://tools.ietf.org/html/rfc952

var (
	regexRFC952 = regexp.MustCompile(regexStringRFC952)
	hostname    string
)

func IsValid(s string) bool {
	return regexRFC952.MatchString(s)
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
//   func Test_something(t *testing.T) {
//     SetHostnameForGoTest("newhostname")
//     defer SetHostnameForGoTest("")
//     // test...
//   }
func SetHostnameForGoTest(s string) {
	hostname = s
}
