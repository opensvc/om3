package patches

import (
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
)

type (
	Patch struct {
		Number      string    `json:"num"`
		Revision    string    `json:"revision"`
		InstalledAt time.Time `json:"installed_at"`
	}
	Patches []Patch
	Lister  func() (Patches, error)
)

// List returns the list of installed patches of all known types
func List() (Patches, error) {
	l := make(Patches, 0)
	for _, fn := range []Lister{ListSolaris} {
		if more, err := fn(); err != nil {
			return l, err
		} else {
			l = append(l, more...)
		}
	}
	return l, nil
}

// ListSolaris returns the solaris patches installed.
// Example output:
//
//	Patch: patchnum-rev Obsoletes: num-rev[,patch-rev]... Requires: .... Incompatibles: ... Packages: ...
func ListSolaris() (Patches, error) {
	l := make(Patches, 0)
	if !file.Exists("/var/sadm/patch") {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Fields(line)
		if len(v) < 4 {
			return
		}
		w := strings.Split(v[1], "-")
		if len(w) != 2 {
			return
		}
		p := Patch{
			Number:   w[0],
			Revision: w[1],
		}
		path := fmt.Sprintf("/var/sadm/patch/%s", p.Number)
		p.InstalledAt = file.ModTime(path)
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("showrev"),
		command.WithVarArgs("-p"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}
