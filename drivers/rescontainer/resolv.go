package rescontainer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type (
	// ResolvConf is the resolver configuration handed to a container.
	ResolvConf struct {
		// Nameservers are written one "nameserver" line each, in order.
		Nameservers []string

		// Searches are written as the single "search" line, in order.
		Searches []string

		// Options are written as the single "options" line, in order.
		Options []string
	}
)

// ResolvConfOptions are the resolver options a container is given, the same
// ones the "--dns-opt" arguments used to carry.
var ResolvConfOptions = []string{"ndots:2", "edns0", "use-vc"}

// MaxNameservers is the number of nameservers a resolver reads. glibc and musl
// both stop at MAXNS, and the lines past it are dead weight in the file.
//
// The search list is not capped the same way: the implementations disagree on
// its limit, glibc counting domains and musl counting bytes, so the file says
// what was asked and each resolver applies its own rule.
const MaxNameservers = 3

// SearchDomains returns the search list of an object, which is its domain and
// each of the parents of that domain.
//
// The domain of "root/svc/svc1" in cluster "clu" is "root.svc.clu", so a name
// is looked up in the namespace of the object, then in its kind, then in the
// cluster. The extra domains come first, so a configured one wins.
func SearchDomains(objectDomain string, extra []string) []string {
	l := make([]string, 0, len(extra)+3)
	l = append(l, extra...)
	for domain := objectDomain; domain != ""; {
		l = append(l, domain)
		_, parent, found := strings.Cut(domain, ".")
		if !found {
			break
		}
		domain = parent
	}
	return l
}

// String returns the file content.
func (t ResolvConf) String() string {
	var sb strings.Builder
	if len(t.Searches) > 0 {
		sb.WriteString("search " + strings.Join(t.Searches, " ") + "\n")
	}
	for i, nameserver := range t.Nameservers {
		if i >= MaxNameservers {
			break
		}
		sb.WriteString("nameserver " + nameserver + "\n")
	}
	if len(t.Options) > 0 {
		sb.WriteString("options " + strings.Join(t.Options, " ") + "\n")
	}
	return sb.String()
}

// IsZero returns true when the resolver configuration would say nothing, and
// the container is better left with the one its image carries.
func (t ResolvConf) IsZero() bool {
	return len(t.Nameservers) == 0 && len(t.Searches) == 0
}

// WriteResolvConf writes the resolver configuration to path and returns it.
//
// The file is written where a container mounts it from, and the container
// reads it for as long as it runs: a container adapts to a cluster layout
// change by being restarted, which is when this is written again.
func WriteResolvConf(path string, resolvConf ResolvConf) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", fmt.Errorf("resolv.conf dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(resolvConf.String()), 0644); err != nil {
		return "", fmt.Errorf("resolv.conf: %w", err)
	}
	return path, nil
}
