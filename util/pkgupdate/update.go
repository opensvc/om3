// Package pkgupdate upgrades the opensvc packages installed on the node: from
// the repositories its package manager has configured when they offer them,
// else from the opensvc package repository, the latest version downloaded and
// installed with the package manager.
package pkgupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultBase is the opensvc package repository.
	DefaultBase = "https://packages.opensvc.com"

	// DefaultBranch is the branch of the packages when none is set: the stable one.
	DefaultBranch = "prod"
)

// Packages are the opensvc packages the update upgrades, those installed.
var Packages = []string{"opensvc-server", "opensvc-client", "opensvc-webapp"}

type (
	// T is an update of the opensvc packages.
	T struct {
		// Base is the url of the package repository: DefaultBase when empty.
		Base string

		// Branch is the branch of the packages: DefaultBranch when empty.
		Branch string

		// DryRun says what the update would do, and changes nothing: the
		// metadata of the configured repositories are not refreshed.
		DryRun bool

		// Out receives what the update does, and the output of the
		// commands it runs.
		Out io.Writer

		// OSRelease is the os-release file: /etc/os-release when empty.
		OSRelease string

		// Client fetches the repository metadata and the packages.
		Client *http.Client

		// TmpDir holds the packages downloaded: os.TempDir() when empty.
		TmpDir string

		// Manager is the package manager: the one of the system
		// (DetectManager) when nil.
		Manager Manager
	}

	installed struct {
		name    string
		version string
	}
)

func (t *T) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(t.Out, format, args...)
}

// Run upgrades the opensvc packages installed. The repositories configured on
// the node are preferred: the packages one of them offers are upgraded from
// there. When none offers any, the latest version of each package is taken
// from the opensvc package repository of the branch and of the distribution,
// and installed with the package manager, when newer than the one installed.
func (t *T) Run(ctx context.Context) error {
	if t.Out == nil {
		t.Out = io.Discard
	}
	mgr := t.Manager
	if mgr == nil {
		if os.Geteuid() != 0 && !t.DryRun {
			return fmt.Errorf("updating the packages requires root")
		}
		var err error
		if mgr, err = DetectManager(); err != nil {
			return err
		}
	}
	pkgs, err := installedPackages(mgr)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("none of the packages %s is installed: install them with %s first", strings.Join(Packages, ", "), mgr.Name())
	}
	if !t.DryRun {
		t.printf("refresh the metadata of the %s repositories\n", mgr.Name())
		if err := t.exec(ctx, mgr, mgr.Refresh(), false); err != nil {
			return err
		}
	}
	var offered, others []installed
	for _, p := range pkgs {
		ok, err := mgr.Offered(p.name)
		if err != nil {
			return err
		}
		if ok {
			offered = append(offered, p)
		} else {
			others = append(others, p)
		}
	}
	if len(offered) > 0 {
		return t.upgrade(ctx, mgr, offered, others)
	}
	return t.download(ctx, mgr, pkgs)
}

func installedPackages(mgr Manager) ([]installed, error) {
	var l []installed
	for _, name := range Packages {
		version, err := mgr.Installed(name)
		if err != nil {
			return nil, err
		}
		if version != "" {
			l = append(l, installed{name: name, version: version})
		}
	}
	return l, nil
}

// upgrade upgrades the packages the configured repositories offer.
func (t *T) upgrade(ctx context.Context, mgr Manager, pkgs, others []installed) error {
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.name
		t.printf("%s %s installed, offered by the configured repositories\n", p.name, p.version)
	}
	for _, p := range others {
		t.printf("%s %s installed, offered by no configured repository: left as it is\n", p.name, p.version)
	}
	return t.exec(ctx, mgr, mgr.Upgrade(names), true)
}

// download installs the latest packages of the opensvc package repository,
// those newer than the installed ones.
func (t *T) download(ctx context.Context, mgr Manager, pkgs []installed) error {
	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	base, repo, err := t.repository(mgr)
	if err != nil {
		return err
	}
	t.printf("no configured repository offers the opensvc packages: get them from %s\n", repo)
	var index []Package
	switch mgr.Format() {
	case "deb":
		id, suite, _ := t.aptRepository()
		index, err = AptIndex(ctx, client, base+"/apt/"+id, suite, DebArch())
	default:
		index, err = RPMIndex(ctx, client, repo, RPMArch())
	}
	if err != nil {
		return err
	}
	var todo []Package
	for _, p := range pkgs {
		latest, ok := Latest(index, p.name, mgr.Compare)
		switch {
		case !ok:
			t.printf("%s %s installed, not in the repository: left as it is\n", p.name, p.version)
		case mgr.Compare(latest.Version, p.version) <= 0:
			t.printf("%s %s installed, up to date\n", p.name, p.version)
		default:
			t.printf("%s %s installed, upgrade to %s\n", p.name, p.version, latest.Version)
			todo = append(todo, latest)
		}
	}
	if len(todo) == 0 || t.DryRun {
		return nil
	}
	dir, err := os.MkdirTemp(t.TmpDir, "opensvc-updatepkg-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	files := make([]string, 0, len(todo))
	for _, p := range todo {
		root := repo
		if mgr.Format() == "deb" {
			// The Filename of an apt package is relative to the os directory.
			id, _, _ := t.aptRepository()
			root = base + "/apt/" + id
		}
		u, err := url.JoinPath(root, p.Path)
		if err != nil {
			return err
		}
		file := filepath.Join(dir, path.Base(p.Path))
		t.printf("download %s\n", u)
		if err := downloadFile(ctx, client, u, file, p.SHA256); err != nil {
			return err
		}
		files = append(files, file)
	}
	if mgr.Format() == "rpm" {
		if err := t.importKey(ctx, client, mgr, base, dir); err != nil {
			return err
		}
	}
	return t.exec(ctx, mgr, mgr.Install(files), true)
}

// importKey imports in the rpm database the key the packages of the
// repository are signed with, which it publishes as gpg.public.key.asc: a
// package manager checking the signatures of the files it installs, as
// zypper, refuses a key it does not know. A repository publishing none is
// said, and its packages left to the checks of the package manager.
func (t *T) importKey(ctx context.Context, client *http.Client, mgr Manager, base, dir string) error {
	u := base + "/gpg.public.key.asc"
	file := filepath.Join(dir, "gpg.public.key.asc")
	body, err := fetch(ctx, client, u)
	if err != nil {
		t.printf("no signing key at %s: %s\n", u, err)
		return nil
	}
	defer func() { _ = body.Close() }()
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(body, 1<<20)); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	t.printf("import the signing key %s\n", u)
	return t.exec(ctx, mgr, []string{"rpm", "--import", file}, false)
}

func (t *T) base() string {
	if t.Base == "" {
		return DefaultBase
	}
	return strings.TrimRight(t.Base, "/")
}

func (t *T) branch() string {
	if t.Branch == "" {
		return DefaultBranch
	}
	return t.Branch
}

func (t *T) distro() (Distro, error) {
	p := t.OSRelease
	if p == "" {
		p = "/etc/os-release"
	}
	return ReadDistro(p)
}

func (t *T) aptRepository() (string, string, error) {
	d, err := t.distro()
	if err != nil {
		return "", "", err
	}
	return d.AptRepository(t.branch())
}

// repository returns the base url and the url of the repository of the
// packages of the branch for the distribution, as the package manager reads it.
func (t *T) repository(mgr Manager) (string, string, error) {
	base := t.base()
	if mgr.Format() == "deb" {
		id, suite, err := t.aptRepository()
		if err != nil {
			return "", "", err
		}
		return base, fmt.Sprintf("%s/apt/%s %s", base, id, suite), nil
	}
	d, err := t.distro()
	if err != nil {
		return "", "", err
	}
	dir, err := d.RPMRepository(t.branch())
	if err != nil {
		return "", "", err
	}
	return base, fmt.Sprintf("%s/rpm/%s/%s", base, dir, RPMArch()), nil
}

// downloadFile writes the file at u to file, checking its sha256 against the
// one the repository metadata give.
func downloadFile(ctx context.Context, client *http.Client, u, file, sum string) error {
	if sum == "" {
		return fmt.Errorf("%s: no sha256 in the repository metadata", u)
	}
	body, err := fetch(ctx, client, u)
	if err != nil {
		return err
	}
	defer func() { _ = body.Close() }()
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), body); err != nil {
		_ = f.Close()
		return fmt.Errorf("%s: %w", u, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, sum) {
		return fmt.Errorf("%s: sha256 %s, the repository says %s", u, got, sum)
	}
	return nil
}

// exec runs a command of the package manager, its output to Out. A command
// changing the packages (detach) runs, when systemd manages the node, in a
// scope of its own: upgrading opensvc-server restarts the daemon, which stops
// the processes of its cgroup, this one included when the daemon runs it as
// a queued action, and the package manager must not be stopped halfway.
func (t *T) exec(ctx context.Context, mgr Manager, argv []string, detach bool) error {
	if detach {
		argv = detachedArgv(argv)
	}
	if t.DryRun {
		t.printf("would run: %s\n", strings.Join(argv, " "))
		return nil
	}
	t.printf("run: %s\n", strings.Join(argv, " "))
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(cmd.Environ(), mgr.Env()...)
	cmd.Stdout, cmd.Stderr = t.Out, t.Out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return nil
}

// detachedArgv is detached, replaced in the tests.
var detachedArgv = detached

// detached prefixes argv to run it out of the cgroup of the caller, in a
// transient scope of systemd, when systemd manages the node.
func detached(argv []string) []string {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return argv
	}
	if _, err := lookPath("systemd-run"); err != nil {
		return argv
	}
	unit := fmt.Sprintf("opensvc-updatepkg-%d", os.Getpid())
	return append([]string{"systemd-run", "--scope", "--quiet", "--unit", unit, "--description", "opensvc packages update", "--"}, argv...)
}
