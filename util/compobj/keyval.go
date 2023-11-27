package main

import (
	"encoding/json"
	"fmt"
)

type (
	CompKeyvals struct {
		*Obj
	}
	CompKeyval struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value string `json:"value"`
	}
)

var (
	keyvalValidityMap = map[string]string{}
	compKeyvalInfo    = ObjInfo{
		DefaultPrefix: "OSVC_COMP_KEYVAL_",
		ExampleValue: []CompKeyval{{
			Key:   "PermitRootLogin",
			Op:    "=",
			Value: "yes",
		}, {
			Key:   "PermitRootLogin",
			Op:    "reset",
			Value: "",
		},
		},
		Description: `* Setup and verify keys in "key value" formatted configuration file.
* Example files: sshd_config, ssh_config, ntp.conf, ...
`,
		FormDefinition: `Desc: |
  A rule to set a list of parameters in simple keyword/value configuration file format. Current values can be checked as set or unset, strictly equal, or superior/inferior to their target value. By default, this object appends keyword/values not found, potentially creating duplicates. The 'reset' operator can be used to avoid such duplicates.
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: keyval
Inputs:
  -
    Id: key
    Label: Key
    DisplayModeTrim: 64
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help:
  -
    Id: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Default: "="
    Candidates:
      - reset
      - unset
      - "="
      - ">="
      - "<="
      - "IN"
    Help: The comparison operator to use to check the parameter current value. The 'reset' operator can be used to avoid duplicate occurence of the same keyword (insert a key reset after the last key set to mark any additional key found in the original file to be removed). The IN operator verifies the current value is one of the target list member. On fix, if the check is in error, it sets the first target list member. A "IN" operator value must be a JSON formatted list.
  -
    Id: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string or integer
    Help: The configuration file parameter target value.
`,
	}
)

func init() {
	m["keyval"] = NewCompKeyvals
}

func NewCompKeyvals() interface{} {
	return &CompKeyvals{
		Obj: NewObj(),
	}
}

func (t *CompKeyvals) Add(s string) error {
	var data []CompKeyval
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Key == "" {
			t.Errorf("key should be in the dict: %s\n", s)
			return fmt.Errorf("key should be in the dict: %s\n", s)
		}
		if rule.Value == "" && (rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "IN") {
			t.Errorf("value should be set in the dict: %s\n", s)
			return fmt.Errorf("value should be set in the dict: %s\n", s)
		}
		if rule.Op == "" {
			t.Errorf("op should be set in the dict: %s\n", s)
			return fmt.Errorf("op should be set in the dict: %s\n", s)
		}
		if !(rule.Op == "reset" || rule.Op == "unset" || rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "IN") {
			t.Errorf("op should be in: reset, unset, =, >=, <=, IN in dict: %s\n", s)
			return fmt.Errorf("op should be in: reset, unset, =, >=, <=, IN in dict: %s\n", s)
		}
		switch rule.Op {
		case "unset":
			if keyvalValidityMap[rule.Key] == "set" {
				keyvalValidityMap[rule.Key] = "unValid"
			} else if keyvalValidityMap[rule.Key] != "unValid" {
				keyvalValidityMap[rule.Key] = "unset"
			}
		case "reset":
			//skip
		default:
			if keyvalValidityMap[rule.Key] == "unset" {
				keyvalValidityMap[rule.Key] = "unValid"
			} else if keyvalValidityMap[rule.Key] != "unValid" {
				keyvalValidityMap[rule.Key] = "set"
			}
		}
		t.Obj.Add(rule)
	}
	t.filterRules()
	return nil
}

func (t *CompKeyvals) filterRules() {
	blacklisted := map[string]any{}
	newRules := []interface{}{}
	for _, rule := range t.Rules() {
		if keyvalValidityMap[rule.(CompKeyval).Key] != "unValid" {
			newRules = append(newRules, rule)
		} else if _, ok := blacklisted[rule.(CompKeyval).Key]; !ok {
			blacklisted[rule.(CompKeyval).Key] = nil
			t.Errorf("the key %s generate some conflicts (asking for a comparison operator and unset at the same time) the key is now blacklisted\n", rule.(CompKeyval).Key)
		}
	}
	t.Obj.rules = newRules
}

func (t CompKeyvals) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	/*	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		o := t.CheckSymlink(rule)
		e = e.Merge(o)
	}*/
	return e
}

func (t CompKeyvals) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	/*	for _, i := range t.Rules() {
		rule := i.(CompSymlink)
		e = e.Merge(t.fixSymlink(rule))
	}*/
	return e
}

func (t CompKeyvals) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompKeyvals) Info() ObjInfo {
	return compKeyvalInfo
}
