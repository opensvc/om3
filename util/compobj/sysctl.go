package main

import (
	"encoding/json"
	"fmt"
)

type (
	CompSysctls struct {
		*Obj
	}
	CompSysctl struct {
		Key   string `json:"key"`
		Index *int   `json:"index"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

var compSysctlInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_SYSCTL_",
	ExampleValue: CompSysctl{
		Key:   "vm.lowmem_reserve_ratio",
		Index: pti(1),
		Op:    "=",
		Value: 256,
	},
	Description: `* Verify a linux kernel parameter value is on target
* Live parameter value (sysctl executable)
* Persistent parameter value (/etc/sysctl.conf)
`,
	FormDefinition: `Desc: |
  A rule to set a list of Linux kernel parameters to be set in /etc/sysctl.conf. Current values can be checked as strictly equal, superior or equal, inferior or equal to their target value. Each field in a vectored value can be tuned independantly using the index key.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: sysctl

Inputs:
  -
    Id: key
    Label: Key
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The /etc/sysctl.conf parameter to check.

  -
    Id: index
    Label: Index
    DisplayModeLabel: idx
    LabelCss: action16
    Mandatory: Yes
    Default: 0
    Type: integer
    Help: The /etc/sysctl.conf parameter to check.

  -
    Id: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Default: "="
    Candidates:
      - "="
      - ">="
      - "<="
    Help: The comparison operator to use to check the parameter current value.

  -
    Id: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string or integer
    Help: The /etc/sysctl.conf parameter target value.
`,
}

func init() {
	m["sysctl"] = NewCompSysctls
}

func NewCompSysctls() interface{} {
	return &CompSysctls{
		Obj: NewObj(),
	}
}

func (t *CompSysctls) Add(s string) error {
	var data []CompSysctl
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Key == "" {
			return fmt.Errorf("key is mandatory in dict : %s \n", s)
		}
		if rule.Index == nil {
			return fmt.Errorf("index is mandatory in dict : %s \n", s)
		}
		if *rule.Index < 0 {
			return fmt.Errorf("index must not be <0 in dict : %s \n", s)
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=") {
			return fmt.Errorf("operator must be =, >= or <= in dict : %s \n", s)
		}
		if rule.Value == nil {
			return fmt.Errorf("value must be in dict : %s \n", s)
		}
		t.Obj.Add(rule)
	}
	return nil
}

/*func (t CompSysctls) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSysctl)
		o := t.CheckSymlink(rule)
		e = e.Merge(o)
	}
	return e
}*/

/*func (t CompSysctls) Fix() ExitCode {
	t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		if e := t.FixSymlink(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}*/

func (t CompSysctls) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompSysctls) Info() ObjInfo {
	return compSysctlInfo
}
