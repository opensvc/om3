package packages

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"
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

// List returns the list of installed packages of all known types
// (deb, rpm, snap, ...).
func List() (Pkgs, error) {
	l := make(Pkgs, 0)
	for _, fn := range []Lister{ListDeb, ListRpm, ListSnap, ListIPS, ListPkginfo, ListPkgutil, ListLPP, ListFreeBSDPkgInfo, ListFreeBSDPkgQuery} {
		if more, err := fn(); err != nil {
			return l, err
		} else {
			l = append(l, more...)
		}
	}
	return l, nil
}

// ListSnap returns the list of installed snap packages.
// Example command output:
//
//	Name      Version    Rev   Tracking  Publisher   Notes
//	core      16-2.35.4  5662  stable    canonical*  core
//	inkscape  0.92.3     4274  stable    inkscape*   -
//	skype     8.32.0.44  60    stable    skype*      classic
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

// ListRpm returns the list of installed rpm packages.
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
			Sig:     v[len(v)-1],
		}
		if ts, err := strconv.ParseInt(v[3], 10, 64); err == nil {
			p.InstalledAt = time.Unix(ts, 0)
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("rpm"),
		command.WithVarArgs("-qa", "--queryformat=%{n} %{v}-%{r} %{arch} %{installtime} %|DSAHEADER?{%{DSAHEADER:pgpsig}}:{%|RSAHEADER?{%{RSAHEADER:pgpsig}}:{%|SIGGPG?{%{SIGGPG:pgpsig}}:{%|SIGPGP?{%{SIGPGP:pgpsig}}:{(none)}|}|}|}|\n"),
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

// ListIPS returns the list of installed ips packages (solaris).
// Example output:
//
//	x11/library/libsm                                 1.2.2-11.4.0.0.1.14.0      i--
func ListIPS() (Pkgs, error) {
	l := make(Pkgs, 0)
	if runtime.GOOS != "solaris" {
		return l, nil
	}
	if _, err := exec.LookPath("pkg"); err != nil {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Fields(line)
		if len(v) != 3 {
			return
		}
		p := Pkg{
			Name:    v[0],
			Version: v[1],
			Arch:    runtime.GOARCH,
			Type:    "ips",
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("pkg"),
		command.WithVarArgs("list", "-H"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

// ListPkginfo returns the list of installed pkginfo packages (solaris).
// Example output:
//
//	 PKGINST:  SUNWzoneu
//	    NAME:  Solaris Zones (Usr)
//	CATEGORY:  system
//	    ARCH:  i386
//	 VERSION:  11.11,REV=2009.04.08.17.26
//	  VENDOR:  Sun Microsystems, Inc.
//	    DESC:  Solaris Zones Configuration and Administration
//	 HOTLINE:  Please contact your local service provider
//	  STATUS:  completely installed
func ListPkginfo() (Pkgs, error) {
	l := make(Pkgs, 0)
	if runtime.GOOS != "solaris" {
		return l, nil
	}
	if _, err := exec.LookPath("pkg"); err != nil {
		return l, nil
	}
	p := Pkg{}
	parse := func(line string) {
		v := strings.SplitN(line, ":", 2)
		if len(v) != 2 {
			return
		}
		key := strings.TrimSpace(v[0])
		val := strings.TrimSpace(v[1])
		switch key {
		case "PKGINST":
			p.Name = val
		case "VERSION":
			p.Version = val
		case "ARCH":
			path := fmt.Sprintf("/var/sadm/pkg/%s", p.Name)
			l = append(l, Pkg{
				Name:        p.Name,
				Version:     p.Version,
				Arch:        val,
				Type:        "pkg",
				InstalledAt: file.ModTime(path),
			})

		}
	}
	cmd := command.New(
		command.WithName("pkginfo"),
		command.WithVarArgs("-l"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

// ListPkgutil returns the list of packages installed with pkgutil (darwin).
// Example output:
//
//	$ pkgutil --packages
//	com.apple.pkg.HP_Scan
//	com.apple.pkg.HP_Scan3
//	...
//
// Example output:
//
//	$ pkgutil --pkg-info com.apple.pkg.X11User
//	package-id: com.apple.pkg.X11User
//	version: 10.6.0.1.1.1238328574
//	volume: /
//	location: /
//	install-time: 1285389505
//	groups: com.apple.snowleopard-repair-permissions.pkg-group com.apple.FindSystemFiles.pkg-group
func ListPkgutil() (Pkgs, error) {
	l := make(Pkgs, 0)
	if runtime.GOOS != "darwin" {
		return l, nil
	}
	if _, err := exec.LookPath("pkgutil"); err != nil {
		return l, nil
	}
	parseInfo := func(line string) {
		v := strings.SplitN(line, ": ", 2)
		if len(v) != 2 {
			return
		}
		key := strings.TrimSpace(v[0])
		val := strings.TrimSpace(v[1])
		p := Pkg{}
		switch key {
		case "version":
			p.Version = val
		case "install-time":
			if ts, err := strconv.ParseInt(val, 10, 64); err == nil {
				p.InstalledAt = time.Unix(ts, 0)
			}
		}
	}
	info := func(name string) error {
		cmd := command.New(
			command.WithName("pkgutil"),
			command.WithVarArgs("--pkg-info", name),
			command.WithOnStdoutLine(parseInfo),
		)
		if err := cmd.Run(); err != nil {
			return err
		}
		return nil
	}
	parse := func(line string) {
		p := Pkg{
			Name: line,
			Arch: runtime.GOARCH,
		}
		if err := info(line); err != nil {
			return
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("pkgutil"),
		command.WithVarArgs("--packages"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

// ListLPP returns the list of installed lpp packages (aix).
func ListLPP() (Pkgs, error) {
	l := make(Pkgs, 0)
	if runtime.GOOS != "aix" {
		return l, nil
	}
	if _, err := exec.LookPath("lslpp"); err != nil {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Split(line, ":")
		if len(v) < 5 {
			return
		}
		p := Pkg{
			Name:    strings.ReplaceAll(v[1], "-"+v[2], ""),
			Version: v[2],
			Arch:    runtime.GOARCH,
			Type:    "lpp",
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("lslpp"),
		command.WithVarArgs("-Lc"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

// ListFreeBSDPkgInfo returns the list of installed pkg_info packages (FreeBSD).
func ListFreeBSDPkgInfo() (Pkgs, error) {
	l := make(Pkgs, 0)
	if runtime.GOOS != "freebsd" {
		return l, nil
	}
	if _, err := exec.LookPath("pkg_info"); err != nil {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Fields(line)
		if len(v) < 2 {
			return
		}
		w := strings.Split(v[0], "-")
		version := w[len(w)-1]
		name := strings.Join(w[0:len(w)-1], "-")
		p := Pkg{
			Name:    name,
			Version: version,
			Arch:    runtime.GOARCH,
			Type:    "pkg",
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("pkg_info"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}

// ListFreeBSDPkgQuery returns the list of installed packages (FreeBSD).
func ListFreeBSDPkgQuery() (Pkgs, error) {
	l := make(Pkgs, 0)
	if runtime.GOOS != "freebsd" {
		return l, nil
	}
	if _, err := exec.LookPath("pkg"); err != nil {
		return l, nil
	}
	parse := func(line string) {
		v := strings.Split(line, ";;")
		if len(v) < 3 {
			return
		}
		p := Pkg{
			Name:    v[0],
			Version: v[1],
			Arch:    v[2],
			Type:    "pkg",
		}
		l = append(l, p)
	}
	cmd := command.New(
		command.WithName("pkg"),
		command.WithVarArgs("query", "-a", "%n;;%v;;%q"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return l, err
	}
	return l, nil
}
