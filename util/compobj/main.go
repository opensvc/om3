package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func main() {
	objName := filepath.Base(os.Args[0])
	newObj, ok := m[objName]
	if !ok {
		fmt.Fprintf(os.Stderr, "%s compliance object not found in the core collection\n", objName)
		os.Exit(1)
	}
	if len(os.Args) != 3 {
		syntax()
		os.Exit(1)
	}
	prefix := os.Args[1]
	action := os.Args[2]
	obj := newObj().(I)
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
