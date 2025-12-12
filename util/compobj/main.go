package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	allocMap map[string]func() interface{}

	I interface {
		Add(string) error
		Check() ExitCode
		Fix() ExitCode
		Fixable() ExitCode
		Rules() []interface{}
		Info() ObjInfo
	}
	Obj struct {
		rules   []interface{}
		verbose bool
	}
	ObjInfo struct {
		DefaultPrefix  string      `json:"default_prefix"`
		ExampleValue   interface{} `json:"example_value"`
		ExampleEnv     interface{} `json:"example_env"`
		Description    string      `json:"description"`
		FormDefinition string      `json:"form_definition"`
	}
	ExitCode     int
	exitCodePair [2]ExitCode
)

var (
	m = allocMap{}

	ExitOk            ExitCode = 0
	ExitNok           ExitCode = 1
	ExitNotApplicable ExitCode = 2

	reWildcardHostname      = regexp.MustCompile(`%%HOSTNAME%%`)
	reWildcardShortHostname = regexp.MustCompile(`%%SHORT_HOSTNAME%%`)
	reWildcardEnvVar1       = regexp.MustCompile(`%%ENV:[A-Z_]{1,}[A-Z0-9_]*%%`)
	reWildcardEnvVar2       = regexp.MustCompile(`(%%ENV:)([A-Z_]{1,}[A-Z0-9_]*)(%%)`)

	varNamePrefix = "OSVC_COMP_"
)

func (t ExitCode) Exit() {
	os.Exit(int(t))
}

func (t ExitCode) Merge(o ExitCode) ExitCode {
	pair := exitCodePair{t, o}
	switch {
	case pair.is(ExitOk, ExitOk):
		return ExitOk
	case pair.is(ExitOk, ExitNok):
		return ExitNok
	case pair.is(ExitOk, ExitNotApplicable):
		return ExitOk
	case pair.is(ExitNok, ExitNotApplicable):
		return ExitNok
	case pair.is(ExitNok, ExitNok):
		return ExitNok
	case pair.is(ExitNotApplicable, ExitNotApplicable):
		return ExitNotApplicable
	default:
		return ExitCode(-1)
	}
}

func NewObj() *Obj {
	return &Obj{
		rules: make([]interface{}, 0),
	}
}

func (t ObjInfo) MarkDown() string {
	indent := func(text string) string {
		buff := ""
		scanner := bufio.NewScanner(strings.NewReader(text))
		for scanner.Scan() {
			buff += "    " + scanner.Text() + "\n"
		}
		return buff
	}
	b, _ := json.MarshalIndent(t.ExampleValue, "", "    ")
	example := string(b)
	env, _ := json.MarshalIndent(t.ExampleEnv, "", "    ")
	exampleEnv := string(env)
	s := ""
	s += "Environment variables\n"
	s += "===========\n"
	s += "\n"
	s += "default prefix : "
	s += indent(t.DefaultPrefix) + "\n"
	s += "\n"
	s += "Description\n"
	s += "===========\n"
	s += "\n"
	s += indent(t.Description) + "\n"
	s += "\n"
	if exampleEnv != "null" {
		s += "Example environment\n"
		s += "============\n"
		s += "\n::\n\n"
		s += indent(exampleEnv) + "\n"
		s += "\n"
	}
	s += "Example rule\n"
	s += "============\n"
	s += "\n::\n\n"
	s += indent(example) + "\n"
	s += "\n"
	s += "Form definition\n"
	s += "===============\n"
	s += "\n::\n\n"
	s += indent(t.FormDefinition) + "\n"
	s += "\n"
	return s
}

func (t Obj) Rules() []interface{} {
	return t.rules
}

func (t *Obj) SetVerbose(v bool) {
	t.verbose = v
}

func (t *Obj) Add(i interface{}) {
	t.rules = append(t.rules, i)
}

func (t Obj) VerboseErrorf(format string, va ...interface{}) {
	if !t.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, format, va...)
}

func (t Obj) VerboseInfof(format string, va ...interface{}) {
	if !t.verbose {
		return
	}
	fmt.Printf(format, va...)
}

func (t Obj) Errorf(format string, va ...interface{}) {
	fmt.Fprintf(os.Stderr, format, va...)
}

func (t Obj) Infof(format string, va ...interface{}) {
	fmt.Printf(format, va...)
}

func syntax() {
	fmt.Fprintf(os.Stderr, `Usage of %s:
  <ENV VARS PREFIX> check     report system non-compliance issues with the rules pointed by <ENV VARS PREFIX>
  <ENV VARS PREFIX> fix       fix issues reported by check
  <ENV VARS PREFIX> fixable   report if issues are fixable
  info                        print the compobj manifest
  test                        run the compobj test
`, os.Args[0])
}

func fprintHelp(w io.Writer) {
	_, _ = fmt.Fprint(w, "The compliance objects in this bundle must be called via a symlink.\n\n")
	_, _ = fmt.Fprint(w, "Bundle content:\n")
	for k := range m {
		_, _ = fmt.Fprintf(w, "  %s\n", k)
	}
	_, _ = fmt.Fprint(w, "\n")
}
func (t exitCodePair) is(e0 ExitCode, e1 ExitCode) bool {
	return (t[0] == e0 && t[1] == e1) || (t[1] == e0 && t[0] == e1)
}

func main() {
	if p, err := os.Readlink(os.Args[0]); err != nil || filepath.Base(p) == filepath.Base(os.Args[0]) {
		if err := bundleMain(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		objMain(os.Args, os.Stdout, os.Stderr).Exit()
	}
}
func symlink(oldname, newname string) error {
	err := os.Symlink(oldname, newname)
	switch {
	case errors.Is(err, os.ErrExist):
		if tgt, err := os.Readlink(newname); err != nil {
			return err
		} else if tgt == oldname {
			fmt.Printf("symlink %s %s: already exists\n", oldname, newname)
		} else {
			fmt.Printf("remove symlink %s %s\n", tgt, newname)
			if err := os.Remove(newname); err != nil {
				return err
			}

			if err := symlink(oldname, newname); err != nil {
				return err
			}
		}
	case err != nil:
		return err
	default:
		fmt.Printf("symlink %s %s\n", oldname, newname)
	}
	return nil
}

func install(installDir string, isRelative bool) error {
	target, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	installDirAbs, err := filepath.Abs(installDir)
	if err != nil {
		return err
	}
	base := installDirAbs
	if isRelative {
		var relativeTarget string
		for {
			relativeTarget, err = filepath.Rel(base, target)
			if err == nil {
				break
			}
			if base == string(filepath.Separator) {
				return err
			}
			if relativeTarget == "" {
				return fmt.Errorf("couldn't find a common head between %s and %s", target, installDirAbs)
			}
			base = filepath.Dir(base)
		}
		target = relativeTarget
	}
	for k := range m {
		newname := filepath.Join(base, k)
		if err := symlink(target, newname); err != nil {
			return err
		}
	}
	return nil
}

func bundleMain() error {
	installDirP := flag.String("i", "", "install bundled comp objs as symlinks in a directory")
	isRelativeP := flag.Bool("r", false, "use relative path for the target of the symlink")
	flag.Parse()

	switch {
	case *installDirP != "":
		return install(*installDirP, *isRelativeP)
	default:
		fprintHelp(os.Stderr)
		flag.Usage()
	}
	return nil
}

func objMain(osArgs []string, wOut, wErr io.Writer) ExitCode {
	objName := filepath.Base(osArgs[0])
	newObj, ok := m[objName]
	if !ok {
		fmt.Fprintf(wErr, "%s compliance object not found in the core collection\n", objName)
		return ExitNok
	}
	var prefix, action string
	switch len(osArgs) {
	case 2:
		action = osArgs[1]
	case 3:
		prefix = osArgs[1]
		action = osArgs[2]
	default:
		syntax()
		return ExitNok
	}
	obj := newObj().(I)
	if prefix == "" {
		prefix = obj.Info().DefaultPrefix
	}
	for _, s := range os.Environ() {
		pair := strings.SplitN(s, "=", 2)
		k := pair[0]
		v := pair[1]
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		v = string(subst([]byte(v)))
		if err := obj.Add(v); err != nil {
			_, _ = fmt.Fprintf(wErr, "incompatible data:  %s", err)
			continue
		}
	}

	switch action {
	case "check":
		return obj.Check()
	case "fix":
		return obj.Fix()
	case "fixable":
		return obj.Fixable()
	case "info":
		nfo := obj.Info()
		_, _ = fmt.Fprintln(wOut, nfo.MarkDown())
		return ExitOk
	default:
		_, _ = fmt.Fprintf(wErr, "invalid action: %s\n", action)
		return ExitOk
	}
}

func getFile(url string) ([]byte, error) {
	if strings.HasPrefix(url, "safe") {
		return collectorSafeGetFile(url)
	}
	return getNormalFile(url)
}

func getNormalFile(url string) ([]byte, error) {
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	response, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func subst(b []byte) []byte {
	hn := hostname.Hostname()
	shn := strings.Split(hn, ".")[0]
	b = reWildcardHostname.ReplaceAll(b, []byte(hn))
	b = reWildcardShortHostname.ReplaceAll(b, []byte(shn))
	b = reWildcardEnvVar1.ReplaceAllFunc(b, func(m []byte) []byte {
		parts := reWildcardEnvVar2.FindSubmatch(m)
		varName := string(parts[2])
		if !strings.HasPrefix(varName, "OSVC_") {
			varName = varNamePrefix + varName
		}
		val := os.Getenv(varName)
		return []byte(val)
	})
	return b
}

func backupDir() string {
	sessionID := os.Getenv("OSVC_SESSION_UUID")
	pathVar := os.Getenv("OSVC_PATH_VAR")
	if sessionID == "" || pathVar == "" {
		return ""
	}
	return filepath.Join(pathVar, "compliance_backup", sessionID)
}

func backup(path string) (string, error) {
	if !file.Exists(path) {
		return "", nil
	}
	dir := backupDir()
	if dir == "" {
		return "", nil
	}
	relPath := strings.TrimPrefix(path, string(os.PathSeparator))
	backupFilePath := filepath.Join(dir, relPath)
	if file.Exists(backupFilePath) {
		return "", nil
	}
	dir = filepath.Dir(backupFilePath)
	if !file.Exists(dir) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", fmt.Errorf("create dir %s: %s", dir, err)
		}
	}
	if err := file.Copy(path, backupFilePath); err != nil {
		return "", fmt.Errorf("backup copy of %s => %s: %s", path, dir, err)
	}
	_ = removeOldBackups()
	return backupFilePath, nil
}

func removeOldBackups() error {
	threshold := time.Now().Add(-time.Hour * 24 * 7)
	pathVar := os.Getenv("OSVC_PATH_VAR")
	if pathVar == "" {
		return nil
	}
	pattern := filepath.Join(pathVar, "compliance_backup", "*")
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		fi, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if fi.ModTime().After(threshold) {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	return nil
}
