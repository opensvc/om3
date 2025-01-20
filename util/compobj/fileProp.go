package main

import (
	"encoding/json"
)

type (
	CompFilesProps struct {
		*Obj
	}
	CompFileProp struct {
		Path string      `json:"path"`
		Mode *int        `json:"mode"`
		UID  interface{} `json:"uid"`
		GID  interface{} `json:"gid"`
	}
)

var (
	compFilesPropsInfo = ObjInfo{
		DefaultPrefix: "OSVC_COMP_FILEPROP_",
		ExampleValue: CompFileProp{
			Path: "/some/path/to/file",
			UID:  500,
			GID:  500,
		},
		Description: `* Verify file existence, mode and ownership.
* The collector provides the format with wildcards.
* The module replace the wildcards with contextual values.

In fix() the file is created empty with the right mode & ownership.

Special wildcards::

  %%ENV:VARNAME%%   Any environment variable value
  %%HOSTNAME%%      Hostname
  %%SHORT_HOSTNAME%%    Short hostname
`,
		FormDefinition: `Desc: |
  A fileprop rule, fed to the 'fileprop' compliance object to verify the target file ownership and permissions.
Css: comp48
Outputs:
  -
    Dest: compliance variable
    Class: fileprop
    Type: json
    Format: dict
Inputs:
  -
    Id: path
    Label: Path
    DisplayModeLabel: path
    LabelCss: action16
    Mandatory: Yes
    Help: File path to check the ownership and permissions for.
    Type: string
  -
    Id: mode
    Label: Permissions
    DisplayModeLabel: perm
    LabelCss: action16
    Help: "In octal form. Example: 644"
    Type: integer
  -
    Id: uid
    Label: Owner
    DisplayModeLabel: uid
    LabelCss: guy16
    Help: Either a user ID or a user name
    Type: string or integer
  -
    Id: gid
    Label: Owner group
    DisplayModeLabel: gid
    LabelCss: guy16
    Help: Either a group ID or a group name
    Type: string or integer
`,
	}
)

func init() {
	m["fileprop"] = NewCompFilesProps
}

func NewCompFilesProps() interface{} {
	return &CompFilesProps{
		Obj: NewObj(),
	}
}

func (t *CompFilesProps) Add(s string) error {
	var data CompFileProp
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	t.Obj.Add(data)
	return nil
}

func (t CompFilesProps) FixRule(rule CompFileProp) ExitCode {
	fileobj := CompFiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	rulefile := CompFile{
		Path: rule.Path,
		Mode: rule.Mode,
		UID:  rule.UID,
		GID:  rule.GID,
		Fmt:  nil,
		Ref:  "",
	}
	if e := fileobj.checkPathExistance(rulefile); e == ExitNok {
		if e := fileobj.fixPathExistence(rulefile); e == ExitNok {
			return ExitNok
		}
	}
	if e := fileobj.checkOwnership(rulefile); e == ExitNok {
		if e := fileobj.fixOwnership(rulefile); e == ExitNok {
			return e
		}
	}
	if e := fileobj.checkMode(rulefile); e == ExitNok {
		if e := fileobj.fixMode(rulefile); e == ExitNok {
			return e
		}
	}
	return ExitOk
}

func (t CompFilesProps) CheckRule(rule CompFileProp) ExitCode {
	fileobj := CompFiles{Obj: &Obj{rules: make([]interface{}, 0), verbose: true}}
	rulefile := CompFile{
		Path: rule.Path,
		Mode: rule.Mode,
		UID:  rule.UID,
		GID:  rule.GID,
		Fmt:  nil,
		Ref:  "",
	}
	var e, o ExitCode
	if o = fileobj.checkPathExistance(rulefile); o == ExitNok {
		return ExitNok
	}
	e = e.Merge(o)
	o = fileobj.checkOwnership(rulefile)
	e = e.Merge(o)
	o = fileobj.checkMode(rulefile)
	e = e.Merge(o)
	return e
}

func (t CompFilesProps) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFileProp)
		o := t.CheckRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompFilesProps) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFileProp)
		e = e.Merge(t.FixRule(rule))
	}
	return e
}

func (t CompFilesProps) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompFilesProps) Info() ObjInfo {
	return compFilesPropsInfo
}
