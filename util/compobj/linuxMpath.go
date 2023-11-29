package main

import (
	"encoding/json"
	"fmt"
)

type (
	CompMpaths struct {
		*Obj
	}
	CompMpath struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

var compMpathInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_MPATH_",
	ExampleValue: []CompMpath{
		{
			Key:   "defaults.polling_interval",
			Op:    ">=",
			Value: 20,
		},
		{
			Key:   "device.{HP}.{HSV210.*}.prio",
			Op:    "=",
			Value: "alua",
		},
		{
			Key:   "blacklist.wwid",
			Op:    "=",
			Value: 600600000001,
		},
	},
	Description: `* Setup and verify the Linux native multipath configuration
`,
	FormDefinition: `Desc: |
  A rule to set a list of Linux multipath.conf parameters. Current values can be checked as equal, or superior/inferior to their target value.
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: linux_mpath
Inputs:
  -
    Id: key
    Label: Key
    DisplayModeTrim: 64
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: >
     The multipath.conf parameter to check.
     ex: defaults.polling_interval or
         device.device.{HP}.{HSV210.*} or
         multipaths.multipath.6006000000000000 or
         blacklist.wwid or
         blacklist.device.{HP}.{HSV210.*}
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
    Help: The multipath.conf parameter target value.
`,
}

func init() {
	m["linux_mpath"] = NewCompMpaths
}

func NewCompMpaths() interface{} {
	return &CompMpaths{
		Obj: NewObj(),
	}
}

func (t *CompMpaths) Add(s string) error {
	var data []CompMpath
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Key == "" {
			t.Errorf("key should be in the dict: %s\n", s)
			return fmt.Errorf("symlink should be in the dict: %s\n", s)
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=") {
			t.Errorf("op should be equal to =, >=, or <= in dict: %s\n", s)
			return fmt.Errorf("op should be equal to =, >=, or <= in dict: %s\n", s)
		}
		if rule.Value == nil {
			t.Errorf("value should be in dict: %s\n", s)
			return fmt.Errorf("value should be in dict: %s\n", s)
		}
		if _, ok := rule.Value.(float64); (rule.Op == ">=" || rule.Op == "<=") && !ok {
			t.Errorf("value should be an int when using operators >= or <= in dict: %s\n", s)
			return fmt.Errorf("value should be an int when using operators >= or <= in dict: %s\n", s)
		}
		_, okString := rule.Value.(string)
		_, okFloat64 := rule.Value.(float64)
		if !(okString || okFloat64) {
			t.Errorf("value should be an int or a string in dict: %s\n", s)
			return fmt.Errorf("value should be an int or a string in dict: %s\n", s)
		}
		t.Obj.Add(rule)
	}
	return nil
}

/*func (t CompMpaths) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		o := t.CheckSymlink(rule)
		e = e.Merge(o)
	}
	return e
}*/

/*func (t CompMpaths) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		e = e.Merge(t.fixSymlink(rule))
	}
	return e
}*/

func (t CompMpaths) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompMpaths) Info() ObjInfo {
	return compMpathInfo
}
