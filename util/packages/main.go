package packages

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/file"
)

type (
	Pkg struct {
		Name        string    `json:"name"`
		Version     string    `json:"version"`
		Arch        string    `json:"arch"`
		Type        string    `json:"type"`
		InstalledAt time.Time `json:"installed_at"`
		Sig         string    `json:"sig"`
	}
	Pkgs   []Pkg
	Lister func() (Pkgs, error)
)

//
// List returns the list of installed packages of all known types
// (deb, rpm, snap, ...).
//
func List() (Pkgs, error) {
	l := make(Pkgs, 0)
	for _, fn := range []Lister{ListDeb, ListRpm, ListSnap} {
		if more, err := fn(); err != nil {
			return l, err
		} else {
			l = append(l, more...)
		}
	}
	return l, nil
}

//
// ListSnap returns the list of installed snap packages.
// Example command output:
//   Name      Version    Rev   Tracking  Publisher   Notes
//   core      16-2.35.4  5662  stable    canonical*  core
//   inkscape  0.92.3     4274  stable    inkscape*   -
//   skype     8.32.0.44  60    stable    skype*      classic
//
func ListSnap() (Pkgs, error) {
	l := make(Pkgs, 0)
	if !file.Exists("/var/lib/snapd") {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Fields(line)
		if len(v) < 4 {
			return
		}
		p := Pkg{
			Name:    v[0],
			Version: v[1] + " rev " + v[2],
			Type:    "snap",
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("snap"),
		command.WithVarArgs("list", "--unicode=never", "--color=never"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

//
// ListRpm returns the list of installed rpm packages.
//
func ListRpm() (Pkgs, error) {
	l := make(Pkgs, 0)
	if !file.Exists("/var/lib/rpm") {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Fields(line)
		if len(v) < 4 {
			return
		}
		p := Pkg{
			Name:    v[0],
			Version: v[1],
			Arch:    v[2],
			Type:    "rpm",
		}
		if ts, err := strconv.ParseInt(v[3], 10, 64); err == nil {
			p.InstalledAt = time.Unix(ts, 0)
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("rpm"),
		command.WithVarArgs("-qa", "--queryformat='%{n} %{v}-%{r} %{arch} %{installtime}\n'"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

// ListDeb returns the list of installed deb packages.
func ListDeb() (Pkgs, error) {
	l := make(Pkgs, 0)
	if !file.Exists("/var/lib/dpkg") {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Fields(line)
		if len(v) < 4 {
			return
		}
		if v[0] != "ii" {
			return
		}
		p := Pkg{
			Name:    v[1],
			Version: v[2],
			Arch:    v[3],
			Type:    "deb",
		}
		path := fmt.Sprintf("/var/lib/dpkg/info/%s.list", p.Name)
		p.InstalledAt = file.ModTime(path)
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("dpkg"),
		command.WithVarArgs("-l"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}
