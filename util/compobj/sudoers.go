package main

import "os/exec"

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
	cmd := exec.Command("bash", "-c"+`"`+"echo "+string(content)+" | "+"sudo "+"visudo "+"-s ", "-c ", "-", `"`)
	_, err = cmd.CombinedOutput()
	if err != nil {
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
			t.Errorf("wrong syntax for sudoers file %s, can't fix it\n", rule.Path)
			e = e.Merge(ExitNok)
		}
		e = e.Merge(t.FixRule(rule))
	}
	return e
}

func (t CompSudoerss) Info() ObjInfo {
	return compSudoersInfo
}
