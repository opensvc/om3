package main

import (
	"encoding/json"
	"os"
)

type (
	CompRemovefiles struct {
		*Obj
	}
	CompRemovefile string
)

var (
	compRemovefileInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_REMOVEFILE_",
		ExampleValue: []CompRemovefile{
			"/tmp/foo",
			"/bar/to/delete",
		},
		Description: `* Verify files and file trees are uninstalled
`,
		FormDefinition: `Desc: |
  A rule defining a set of files to remove, fed to the 'remove_files' compliance object.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Class: remove_files
    Type: json
    Format: list

Inputs:
  -
    Id: path
    Label: File path
    DisplayModeLabel: ""
    LabelCss: edit16
    Mandatory: Yes
    Help: You must set paths in fully qualified form.
    Type: string
`,
	}
)

func init() {
	m["remove_files"] = NewCompRemoveFiles
}

func NewCompRemoveFiles() interface{} {
	return &CompRemovefiles{
		Obj: NewObj(),
	}
}

func (t *CompRemovefiles) Add(s string) error {
	var data []CompRemovefile
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, file := range data {
		t.Obj.Add(file)
	}
	return nil
}

func (t CompRemovefiles) checkRule(rule CompRemovefile) ExitCode {
	_, err := os.Stat(string(rule))
	switch {
	case err == nil:
		t.VerboseErrorf("the file %s exist and should not exist\n", rule)
		return ExitNok
	case os.IsNotExist(err):
		t.VerboseInfof("the file %s does not exist and should not exist\n", rule)
		return ExitOk
	default:
		t.Errorf("%s", err)
		return ExitNok
	}
}

func (t CompRemovefiles) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompRemovefile)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompRemovefiles) fixRule(rule CompRemovefile) ExitCode {
	if err := os.Remove(string(rule)); err != nil {
		t.Errorf("%s", err)
		return ExitNok
	}
	t.Infof("remove the file %s\n", rule)
	return ExitOk
}

func (t CompRemovefiles) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompRemovefile)
		if t.checkRule(rule) == ExitNok {
			e = e.Merge(t.fixRule(rule))
		}
	}
	return e
}

func (t CompRemovefiles) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompRemovefiles) Info() ObjInfo {
	return compRemovefileInfo
}
