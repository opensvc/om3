package packages

import (
	"fmt"
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
	Pkgs []Pkg
)

func List() ([]Pkg, error) {
	l := make([]Pkg, 0)
	if more, err := ListDeb(); err != nil {
		return l, err
	} else {
		l = append(l, more...)
	}
	return l, nil
}

func ListDeb() ([]Pkg, error) {
	l := make([]Pkg, 0)
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
