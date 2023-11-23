package main

import (
	"fmt"
	"github.com/goccy/go-json"
)

type (
	CompFileincs struct {
		*Obj
	}
	CompFileinc struct {
		Path      string `json:"path"`
		Check     string `json:"check"`
		Replace   string `json:"replace"`
		Fmt       string `json:"fmt"`
		StrictFmt bool   `json:"strict_Fmt"`
		Ref       string `json:"ref"`
	}
)

var compFileincInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_FILEINC_",
	ExampleValue: CompFileinc{
		Path:  "/tmp/foo",
		Check: ".*some pattern.*",
		Fmt:   "full added content with %%HOSTNAME%%@corp.com: some pattern into the file",
	},
	Description: `* Verify or Change file content.
* The collector provides the format with wildcards.
* The module replace the wildcards with contextual values.
* The fmt must match the check pattern ['check' statement]
* The fmt is used to substitute any string matching the replace pattern ['replace' statement]

Wildcards:
%%ENV:VARNAME%%		Any environment variable value
%%HOSTNAME%%		Hostname
%%SHORT_HOSTNAME%%	Short hostname
`,
	FormDefinition: `
Desc: |
  A fileinc rule, fed to the 'fileinc' compliance object to verify a line matching the 'check' regular expression is present in the specified file. Alternatively, the 'replace' statement can be used to substitute any matching expression by string provided by 'fmt' or 'ref' content.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Class: fileinc
    Type: json
    Format: dict

Inputs:
  -
    Id: path
    Label: Path
    DisplayModeLabel: path
    LabelCss: hd16
    Mandatory: Yes
    Help: File path to search the matching line into.
    Type: string
  -
    Id: check
    Label: Check regexp
    DisplayModeLabel: check
    LabelCss: action16
    Mandatory: No
    Help: A regular expression. Matching the regular expression is sufficent to grant compliancy. It is required to use either 'check' or 'replace'.
    Type: string
  -
    Id: replace
    Label: Replace regexp
    DisplayModeLabel: replace
    LabelCss: action16
    Mandatory: No
    Help: A regular expression. Any pattern matched by the reguler expression will be replaced. It is required to use either 'check' or 'replace'.
    Type: string
  -
    Id: fmt
    Label: Format
    DisplayModeLabel: fmt
    LabelCss: action16
    Help: The line installed if the check pattern is not found in the file.
    Type: string
  -
    Id: strict_fmt
    Label: Strict Format
    DisplayModeLabel: strict fmt
    LabelCss: action16
    Help: Consider a line matching the check regexp invalid if the line is not strictly the same as fmt.
    Type: boolean
    Default: True
  -
    Id: ref
    Label: URL to format
    DisplayModeLabel: ref
    LabelCss: loc
    Help: An URL pointing to a file containing the line installed if the check pattern is not found in the file.
    Type: string
`,
}

func init() {
	m["fileinc"] = NewCompFileincs
}

func NewCompFileincs() interface{} {
	return &CompFileincs{
		Obj: NewObj(),
	}
}

func (t *CompFileincs) Add(s string) error {
	var data CompFileinc
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	if data.Path == "" {
		return fmt.Errorf("path should be in the dict: %s", s)
	}
	if data.Check == "" && data.Replace == "" {
		return fmt.Errorf("check or replace should be in the dict: %s", s)
	}
	if data.Check != "" && data.Replace != "" {
		return fmt.Errorf("check and replace can't be both in the dict: %s", s)
	}
	if data.Fmt != "" && data.Ref != "" {
		return fmt.Errorf("fmt and ref can't be both in the same dict: %s", s)
	}
	t.Obj.Add(data)
	return nil
}

func (t CompFileincs) checkRule(rule CompFileinc) ExitCode {
	return ExitOk
}

func (t CompFileincs) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompFileinc)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompFileincs) Fix() ExitCode {
	/*t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		if e := t.FixSymlink(rule); e == ExitNok {
			return ExitNok
		}
	}*/
	return ExitOk
}

func (t CompFileincs) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompFileincs) Info() ObjInfo {
	return compFileincInfo
}
