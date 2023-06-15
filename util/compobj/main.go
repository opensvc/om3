package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
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
		Description    string      `json:"description"`
		FormDefinition string      `json:"form_definition"`
	}
	ExitCode int
)

var (
	m = allocMap{}

	ExitOk            ExitCode = 0
	ExitNok           ExitCode = 1
	ExitNotApplicable ExitCode = 2

	reWildcardHostname      = regexp.MustCompile(`%%HOSTNAME%%`)
	reWildcardShortHostname = regexp.MustCompile(`%%SHORT_HOSTNAME%%`)
	reWildcardEnvVar1       = regexp.MustCompile(`%%ENV:.+%%`)
	reWildcardEnvVar2       = regexp.MustCompile(`(%%ENV:)(.+)(%%)`)
)

func (t ExitCode) Exit() {
	os.Exit(int(t))
}

func (t ExitCode) Merge(o ExitCode) ExitCode {
	switch {
	case t == ExitOk && o == ExitOk:
		return ExitOk
	case t == ExitOk && o == ExitNok:
		return ExitNok
	case t == ExitOk && o == ExitNotApplicable:
		return ExitOk
	case t == ExitNok && o == ExitOk:
		return ExitNok
	case t == ExitNok && o == ExitNotApplicable:
		return ExitNok
	case t == ExitNotApplicable && o == ExitNotApplicable:
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
	s := ""
	s += "Description\n"
	s += "===========\n"
	s += "\n"
	s += indent(t.Description) + "\n"
	s += "\n"
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
	fmt.Fprintf(os.Stderr, `syntax:
%s <ENV VARS PREFIX> check|fix|fixable
%s test|info
`, os.Args[0], os.Args[0])
}

func links() {
	fmt.Println("The compliance objects in this collection must be called via a symlink.")
	fmt.Println("Collection content:")
	for k, _ := range m {
		fmt.Printf("  %s\n", k)
	}
}

func main() {
	objName := filepath.Base(os.Args[0])
	if p, err := os.Readlink(os.Args[0]); err != nil || filepath.Base(p) == objName {
		links()
		os.Exit(0)
	}
	newObj, ok := m[objName]
	if !ok {
		fmt.Fprintf(os.Stderr, "%s compliance object not found in the core collection\n", objName)
		os.Exit(1)
	}
	var prefix, action string
	switch len(os.Args) {
	case 2:
		action = os.Args[1]
	case 3:
		prefix = os.Args[1]
		action = os.Args[2]
	default:
		syntax()
		os.Exit(1)
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
		if err := obj.Add(v); err != nil {
			fmt.Fprintf(os.Stderr, "incompatible data: %s: %+v\n", err, v)
			continue
		}
	}

	var e ExitCode
	switch action {
	case "check":
		e = obj.Check()
	case "fix":
		e = obj.Fix()
	case "fixable":
		e = obj.Fixable()
	case "info":
		nfo := obj.Info()
		fmt.Println(nfo.MarkDown())
	default:
		fmt.Fprintf(os.Stderr, "invalid action: %s\n", action)
		e.Exit()
	}
	e.Exit()
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
		val := os.Getenv("OSVC_COMP_" + string(parts[2]))
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
