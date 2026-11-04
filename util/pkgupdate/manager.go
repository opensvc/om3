package pkgupdate

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type (
	// Manager is the package manager of the system, the commands it runs to
	// say and change what is installed.
	Manager interface {
		// Name is the name of its command: apt-get, dnf, yum, zypper.
		Name() string

		// Format is the format of its packages: deb or rpm.
		Format() string

		// Installed returns the version of a package installed, empty when
		// it is not.
		Installed(name string) (string, error)

		// Refresh reads again the metadata of the configured repositories.
		Refresh() []string

		// Offered tells whether a configured repository offers the package.
		Offered(name string) (bool, error)

		// Upgrade upgrades the packages from the configured repositories.
		Upgrade(names []string) []string

		// Install installs package files, their dependencies taken from the
		// configured repositories.
		Install(files []string) []string

		// Env is the environment its commands run with, besides the agent's.
		Env() []string

		// Compare compares two versions of its packages.
		Compare(a, b string) int
	}

	apt    struct{}
	dnf    struct{}
	yum    struct{}
	zypper struct{}
)

var (
	// ErrNoManager is returned on a system with no package manager the
	// update knows.
	ErrNoManager = errors.New("no package manager known: apt-get, dnf, yum or zypper")

	lookPath = exec.LookPath
)

// DetectManager returns the package manager of the system: apt on a debian
// one, else dnf, yum or zypper, the first found.
func DetectManager() (Manager, error) {
	has := func(name string) bool {
		_, err := lookPath(name)
		return err == nil
	}
	switch {
	case has("apt-get") && has("dpkg-query"):
		return apt{}, nil
	case !has("rpm"):
		return nil, ErrNoManager
	case has("dnf"):
		return dnf{}, nil
	case has("zypper"):
		return zypper{}, nil
	case has("yum"):
		return yum{}, nil
	}
	return nil, ErrNoManager
}

// output runs a query and returns its standard output; a non-zero exit is no
// error when ok says the code is an answer.
func output(argv []string, env []string, ok ...int) (string, int, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(cmd.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		for _, c := range ok {
			if code == c {
				return stdout.String(), code, nil
			}
		}
		return "", code, fmt.Errorf("%s: %w: %s", strings.Join(argv, " "), err, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return "", -1, fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return stdout.String(), 0, nil
}

// rpmInstalled returns the version of an installed rpm package, as
// [epoch:]version-release, empty when not installed.
func rpmInstalled(name string) (string, error) {
	out, code, err := output([]string{"rpm", "-q", "--qf", "%{EPOCH}:%{VERSION}-%{RELEASE}\n", name}, nil, 1)
	if err != nil || code != 0 {
		return "", err
	}
	v := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	return strings.TrimPrefix(v, "(none):"), nil
}

func (apt) Name() string   { return "apt-get" }
func (apt) Format() string { return "deb" }
func (apt) Env() []string {
	return []string{"DEBIAN_FRONTEND=noninteractive", "NEEDRESTART_MODE=a"}
}
func (apt) Compare(a, b string) int { return CompareDeb(a, b) }

func (apt) Installed(name string) (string, error) {
	out, code, err := output([]string{"dpkg-query", "-W", "-f", "${Status}\t${Version}", name}, nil, 1)
	if err != nil || code != 0 {
		return "", err
	}
	status, version, _ := strings.Cut(out, "\t")
	if !strings.HasSuffix(status, " installed") || strings.HasPrefix(status, "deinstall") {
		return "", nil
	}
	return strings.TrimSpace(version), nil
}

func (apt) Refresh() []string { return []string{"apt-get", "-q", "update"} }

// Offered: apt-cache madison lists the versions the configured repositories
// offer, not the installed one.
func (t apt) Offered(name string) (bool, error) {
	out, _, err := output([]string{"apt-cache", "madison", name}, t.Env())
	if err != nil {
		return false, err
	}
	return parseMadison(out, name), nil
}

func parseMadison(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) >= 3 && strings.TrimSpace(fields[0]) == name {
			return true
		}
	}
	return false
}

var aptOptions = []string{"-y", "-q", "-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold"}

func (apt) Upgrade(names []string) []string {
	argv := append([]string{"apt-get", "install", "--only-upgrade"}, aptOptions...)
	return append(argv, names...)
}

func (apt) Install(files []string) []string {
	argv := append([]string{"apt-get", "install"}, aptOptions...)
	return append(argv, files...)
}

func (dnf) Name() string                          { return "dnf" }
func (dnf) Format() string                        { return "rpm" }
func (dnf) Env() []string                         { return nil }
func (dnf) Compare(a, b string) int               { return CompareRPM(a, b) }
func (dnf) Installed(name string) (string, error) { return rpmInstalled(name) }
func (dnf) Refresh() []string                     { return []string{"dnf", "-q", "makecache"} }

// Offered: dnf repoquery lists the packages the enabled repositories offer.
func (dnf) Offered(name string) (bool, error) {
	out, _, err := output([]string{"dnf", "-q", "repoquery", "--qf", "%{name}\n", name}, nil)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func (dnf) Upgrade(names []string) []string {
	return append([]string{"dnf", "-y", "upgrade"}, names...)
}

func (dnf) Install(files []string) []string {
	return append([]string{"dnf", "-y", "install"}, files...)
}

func (yum) Name() string                          { return "yum" }
func (yum) Format() string                        { return "rpm" }
func (yum) Env() []string                         { return nil }
func (yum) Compare(a, b string) int               { return CompareRPM(a, b) }
func (yum) Installed(name string) (string, error) { return rpmInstalled(name) }
func (yum) Refresh() []string                     { return []string{"yum", "-q", "makecache"} }

// Offered: yum list shows, besides the installed version (repository
// "installed" or "@<repo>"), those the enabled repositories offer.
func (yum) Offered(name string) (bool, error) {
	out, _, err := output([]string{"yum", "-q", "--showduplicates", "list", name}, nil, 1)
	if err != nil {
		return false, err
	}
	return parseYumList(out, name), nil
}

func parseYumList(out, name string) bool {
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || !strings.HasPrefix(fields[0], name+".") {
			continue
		}
		repo := fields[2]
		if repo != "installed" && !strings.HasPrefix(repo, "@") {
			return true
		}
	}
	return false
}

func (yum) Upgrade(names []string) []string {
	return append([]string{"yum", "-y", "update"}, names...)
}

func (yum) Install(files []string) []string {
	return append([]string{"yum", "-y", "localinstall"}, files...)
}

func (zypper) Name() string                          { return "zypper" }
func (zypper) Format() string                        { return "rpm" }
func (zypper) Env() []string                         { return nil }
func (zypper) Compare(a, b string) int               { return CompareRPM(a, b) }
func (zypper) Installed(name string) (string, error) { return rpmInstalled(name) }
func (zypper) Refresh() []string {
	return []string{"zypper", "--non-interactive", "--quiet", "refresh"}
}

// Offered: zypper search lists the package in each repository offering it,
// the installed one in "(System Packages)" when no repository does.
func (zypper) Offered(name string) (bool, error) {
	out, _, err := output([]string{"zypper", "--non-interactive", "--quiet", "search", "--match-exact", "-s", "-t", "package", name}, nil, 104)
	if err != nil {
		return false, err
	}
	return parseZypperSearch(out, name), nil
}

func parseZypperSearch(out, name string) bool {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 6 || strings.TrimSpace(fields[1]) != name {
			continue
		}
		if repo := strings.TrimSpace(fields[len(fields)-1]); repo != "" && repo != "(System Packages)" {
			return true
		}
	}
	return false
}

func (zypper) Upgrade(names []string) []string {
	return append([]string{"zypper", "--non-interactive", "update"}, names...)
}

func (zypper) Install(files []string) []string {
	return append([]string{"zypper", "--non-interactive", "install"}, files...)
}
