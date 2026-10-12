package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

type (
	CompSudoerss struct {
		CompFiles
	}
)

var compSudoersInfo = ObjInfo{
	Description: `Same as files compliance object, but verifies the sudoers
declaration syntax using visudo in check mode.

The variable format is json-serialized:

{
  "path": "/some/path/to/file",
  "fmt": "root@corp.com		%%HOSTNAME%%@corp.com",
  "uid": 500,
  "gid": 500,
}

Wildcards:
%%ENV:VARNAME%%		Any environment variable value
%%HOSTNAME%%		Hostname
%%SHORT_HOSTNAME%%	Short hostname
`,
}

func init() {
	m["sudoers"] = NewCompSudoerss
}

func NewCompSudoerss() interface{} {
	return &CompSudoerss{
		CompFiles{NewObj()},
	}
}

func (t CompSudoerss) checkSyntax(rule CompFile) ExitCode {
	content, err := rule.Content()
	if err != nil {
		t.Errorf("%s\n", err)
		return ExitNok
	}
	// The content is given to visudo on its stdin: written in a shell
	// command line, a quote in it would end the quoting.
	cmd := exec.Command("visudo", "-s", "-c", "-")
	cmd.Stdin = bytes.NewReader(content)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output), err)
		t.VerboseErrorf("wrong syntax for sudoers file %s\n", rule.Path)
		return ExitNok
	}
	return ExitOk
}

func (t CompSudoerss) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFile)
		e = e.Merge(t.checkSyntax(rule))
		e = e.Merge(t.CheckRule(rule))
	}
	return e
}

func (t CompSudoerss) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFile)
		if t.checkSyntax(rule) == ExitNok {
			// A file visudo refuses would break sudo: it is not installed.
			t.Errorf("wrong syntax for sudoers file %s, can't fix it\n", rule.Path)
			e = e.Merge(ExitNok)
			continue
		}
		e = e.Merge(t.FixRule(rule))
	}
	return e
}

func (t CompSudoerss) Info() ObjInfo {
	return compSudoersInfo
}
