package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/xmap"
)

type (
	CompPackages struct {
		*Obj
	}
	CompPackage []string
)

var compPackagesInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_PACKAGES_",
	ExampleValue: CompPackage{
		"bzip2",
		"-zip",
		"zip",
	},
	Description: `* Verify a list of packages is installed or removed
* A '-' prefix before the package name means the package should be removed
* No prefix before the package name means the package should be installed
* The package version is not checked
`,
	FormDefinition: `Desc: |
  A rule defining a set of packages, fed to the 'packages' compliance object for it to check each package installed or not-installed status.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Class: package
    Type: json
    Format: list

Inputs:
  -
    Id: pkgname
    Label: Package name
    DisplayModeLabel: ""
    LabelCss: pkg16
    Mandatory: Yes
    Help: Use '-' as a prefix to set 'not installed' as the target state. Use '*' as a wildcard for package name expansion for operating systems able to list packages available for installation.
    Type: string
`,
}

var (
	packages = map[string]interface{}{}
	hasItMap = map[string]bool{}
	osVendor = os.Getenv("OSVC_COMP_NODES_OS_VENDOR")
	osName   = os.Getenv("OSVC_COMP_NODES_OS_NAME")
)

func init() {
	m["package"] = NewCompPackages
}

func hasDpkg() bool {
	return hasIt("dpkg", func() bool {
		if osName != "Linux" {
			return false
		}
		switch osVendor {
		case "Ubuntu", "Debian":
			// pass
		default:
			return false
		}
		p, err := exec.LookPath("dpkg")
		return p != "" && err == nil
	})
}

func hasYum() bool {
	return hasIt("yum", func() bool {
		if osName != "Linux" {
			return false
		}
		switch osVendor {
		case "Red Hat", "RedHat", "CentOS", "Oracle":
			// pass
		default:
			return false
		}
		p, err := exec.LookPath("yum")
		return p != "" && err == nil
	})
}

func hasRpm() bool {
	return hasIt("rpm", func() bool {
		if osName != "Linux" {
			return false
		}
		switch osVendor {
		case "Red Hat", "RedHat", "CentOS", "Oracle", "SuSE":
			// pass
		default:
			return false
		}
		p, err := exec.LookPath("rpm")
		return p != "" && err == nil
	})
}

func hasZypper() bool {
	return hasIt("zypper", func() bool {
		if osName != "Linux" {
			return false
		}
		switch osVendor {
		case "SuSE":
			// pass
		default:
			return false
		}
		p, err := exec.LookPath("zypper")
		return p != "" && err == nil
	})
}

func hasPkgadd() bool {
	return hasIt("pkgadd", func() bool {
		if osName != "SunOS" {
			return false
		}
		p, err := exec.LookPath("pkgadd")
		return p != "" && err == nil
	})
}

func hasApt() bool {
	return hasIt("apt", func() bool {
		if osName != "Linux" {
			return false
		}
		switch osVendor {
		case "Ubuntu", "Debian":
			// pass
		default:
			return false
		}
		p, err := exec.LookPath("apt")
		return p != "" && err == nil
	})
}

func hasFreebsdPkg() bool {
	return hasIt("freebsdpkg", func() bool {
		if osName != "FreeBSD" {
			return false
		}
		p, err := exec.LookPath("pkg")
		return p != "" && err == nil
	})
}

func hasApk() bool {
	return hasIt("apk", func() bool {
		if osName != "Linux" {
			return false
		}
		switch osVendor {
		case "Alpine":
			// pass
		default:
			return false
		}
		p, err := exec.LookPath("apk")
		return p != "" && err == nil
	})
}

func hasIt(key string, fn func() bool) bool {
	if v, ok := hasItMap[key]; ok {
		return v
	}
	v := fn()
	hasItMap[key] = v
	return v
}

func NewCompPackages() interface{} {
	return &CompPackages{
		Obj: NewObj(),
	}
}

func (t *CompPackages) Add(s string) error {
	data := make(CompPackage, 0)
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	t.Obj.Add(data)
	return nil
}

func loadInstalledPackages() error {
	if osVendor == "" {
		return errors.Errorf("OSVC_COMP_NODES_OS_VENDOR not set")
	}
	if osName == "" {
		return errors.Errorf("OSVC_COMP_NODES_OS_NAME not set")
	}
	var err error
	switch {
	case hasDpkg():
		err = dpkgLoadInstalledPackages()
	case hasRpm():
		err = rpmLoadInstalledPackages()
	case hasPkgadd():
		err = pkginfoLoadInstalledPackages()
	case hasFreebsdPkg():
		err = freebsdPkgLoadInstalledPackages()
	default:
		return errors.Errorf("unsupported os")
	}
	return err
}

func fo(s string) {
	fmt.Println(s)
}

func fe(s string) {
	fmt.Fprintln(os.Stderr, s)
}

func freebsdPkgAdd(names []string) error {
	args := []string{"install", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("pkg"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func freebsdPkgDel(names []string) error {
	args := []string{"remove", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("pkg"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func apkAdd(names []string) error {
	args := []string{"add", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("apk"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func apkDel(names []string) error {
	args := []string{"del", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("apk"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func zypperAdd(names []string) error {
	args := []string{"install", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("zypper"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func zypperDel(names []string) error {
	args := []string{"remove", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("zypper"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func yumAdd(names []string) error {
	names, err := yumExpand(names)
	if err != nil {
		return err
	}
	args := []string{"-y", "install"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("yum"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func yumDel(names []string) error {
	args := []string{"-y", "remove"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("yum"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func yumExpand(names []string) ([]string, error) {
	args := []string{"list"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("yum"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(fe),
	)
	err := cmd.Run()
	if err != nil {
		return names, err
	}
	expanded := map[string]interface{}{}
	scanner := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for scanner.Scan() {
		line := string(scanner.Text())
		l := strings.Fields(line)
		if len(l) != 3 {
			continue
		}
		name := l[0]
		expanded[name] = nil
	}
	return xmap.Keys(expanded), nil
}

func aptExpand(names []string) ([]string, error) {
	args := []string{"list"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("apt"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(fe),
	)
	err := cmd.Run()
	if err != nil {
		return names, err
	}
	expanded := map[string]interface{}{}
	scanner := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for scanner.Scan() {
		line := string(scanner.Text())
		l := strings.Split(line, "/")
		if len(l) < 2 {
			continue
		}
		name := l[0]
		expanded[name] = nil
	}
	return xmap.Keys(expanded), nil
}

func aptAdd(names []string) error {
	names, err := aptExpand(names)
	if err != nil {
		return err
	}
	args := []string{"install", "--allow-unauthenticated", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("apt-get"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func aptDel(names []string) error {
	args := []string{"remove", "-y"}
	args = append(args, names...)
	cmd := command.New(
		command.WithName("apt-get"),
		command.WithArgs(args),
		command.WithOnStdoutLine(fo),
		command.WithOnStderrLine(fe),
	)
	fmt.Println(cmd)
	return cmd.Run()
}

func rpmLoadInstalledPackages() error {
	cmd := command.New(
		command.WithName("rpm"),
		command.WithVarArgs("-qa", "--qf", "%{NAME}.%{ARCH}\n"),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(fe),
	)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "can not fetch installed packages list")
	}
	scanner := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for scanner.Scan() {
		name := string(scanner.Text())
		packages[name] = nil
	}
	return nil
}

func pkginfoLoadInstalledPackages() error {
	cmd := command.New(
		command.WithName("pkginfo"),
		command.WithVarArgs("-l"),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(fe),
	)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "can not fetch installed packages list")
	}
	scanner := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for scanner.Scan() {
		line := string(scanner.Text())
		v := strings.Split(line, ":")
		if len(v) != 2 {
			continue
		}
		if strings.TrimSpace(v[0]) != "PKGINST" {
			continue
		}
		name := strings.TrimSpace(v[1])
		packages[name] = nil
	}
	return nil
}

func dpkgLoadInstalledPackages() error {
	cmd := command.New(
		command.WithName("dpkg"),
		command.WithVarArgs("-l"),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(fe),
	)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "can not fetch installed packages list")
	}
	scanner := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for scanner.Scan() {
		line := string(scanner.Text())
		if !strings.HasPrefix(line, "ii") {
			continue
		}
		name := strings.Fields(line)[1]
		name = strings.Split(name, ":")[0]
		packages[name] = nil
	}
	return nil
}

func freebsdPkgLoadInstalledPackages() error {
	cmd := command.New(
		command.WithName("pkg"),
		command.WithVarArgs("info"),
		command.WithBufferedStdout(),
		command.WithOnStderrLine(fe),
	)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "can not fetch installed packages list")
	}
	scanner := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for scanner.Scan() {
		line := string(scanner.Text())
		l := strings.Fields(line)
		line = l[0]
		l = strings.Split(line, "-")
		line = l[0]
		if line == "" {
			continue
		}
		packages[line] = nil
	}
	return nil
}

func (t CompPackages) fixPkgAdd(names []string) ExitCode {
	var err error
	switch {
	case hasApt():
		err = aptAdd(names)
	default:
		return ExitNotApplicable
	}
	if err != nil {
		t.Errorf("package install: %s\n", err)
		return ExitNok
	}
	return ExitOk
}

func (t CompPackages) fixPkgDel(names []string) ExitCode {
	var err error
	switch {
	case hasApt():
		err = aptDel(names)
	case hasYum():
		err = yumDel(names)
	case hasZypper():
		err = zypperDel(names)
	case hasFreebsdPkg():
		err = freebsdPkgDel(names)
	case hasApk():
		err = apkDel(names)
	default:
		return ExitNotApplicable
	}
	if err != nil {
		t.Errorf("package uninstall: %s\n", err)
		return ExitNok
	}
	return ExitOk
}

func (t CompPackages) checkPkgAdd(name string) ExitCode {
	if _, ok := packages[name]; !ok {
		t.VerboseErrorf("package %s is not installed, but should be\n", name)
		return ExitNok
	}
	t.VerboseInfof("package %s is installed\n", name)
	return ExitOk
}

func (t CompPackages) checkPkgDel(name string) ExitCode {
	if _, ok := packages[name]; ok {
		t.Errorf("package %s is installed, but should not be\n", name)
		return ExitNok
	}
	t.Infof("package %s is not installed\n", name)
	return ExitOk
}

func (t CompPackages) CheckRule(rule CompPackage) ExitCode {
	var e, o ExitCode
	for _, s := range rule {
		s = strings.TrimPrefix(s, "+")
		if strings.HasPrefix(s, "-") {
			s = strings.TrimPrefix(s, "-")
			o = t.checkPkgDel(s)
		} else {
			o = t.checkPkgAdd(s)
		}
		e = e.Merge(o)
	}
	return e
}

func (t CompPackages) Check() ExitCode {
	t.SetVerbose(true)
	if err := loadInstalledPackages(); err != nil {
		t.VerboseErrorf("%s\n", err)
		return ExitNotApplicable
	}
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompPackage)
		o := t.CheckRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompPackages) parseRules() ([]string, []string) {
	adds := []string{}
	dels := []string{}
	for _, i := range t.Rules() {
		rule := i.(CompPackage)
		al, dl := t.parseRule(rule)
		adds = append(adds, al...)
		dels = append(dels, dl...)
	}
	return adds, dels
}

func (t CompPackages) parseRule(rule CompPackage) ([]string, []string) {
	adds := []string{}
	dels := []string{}
	for _, s := range rule {
		s = strings.TrimPrefix(s, "+")
		if strings.HasPrefix(s, "-") {
			s = strings.TrimPrefix(s, "-")
			if o := t.checkPkgDel(s); o == ExitNok {
				dels = append(dels, s)
			}
		} else {
			if o := t.checkPkgAdd(s); o == ExitNok {
				adds = append(adds, s)
			}
		}
	}
	return adds, dels
}

func (t CompPackages) Fix() ExitCode {
	e := ExitNotApplicable
	t.SetVerbose(false)
	if err := loadInstalledPackages(); err != nil {
		t.Errorf("%s\n", err)
		return ExitNotApplicable
	}
	adds, dels := t.parseRules()
	if len(dels) > 0 {
		o := t.fixPkgDel(dels)
		e = e.Merge(o)
	}
	if len(adds) > 0 {
		o := t.fixPkgAdd(adds)
		e = e.Merge(o)
	}
	return e
}

func (t CompPackages) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompPackages) Info() ObjInfo {
	return compPackagesInfo
}
