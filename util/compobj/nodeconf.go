package main

import (
	"encoding/json"
	"fmt"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/rs/zerolog"
)

type (
	CompNodeconfs struct {
		*Obj
	}
	CompNodeconf struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

var (
	ruleNodeConf        = map[string]CompNodeconf{}
	blacklistedNodeConf = map[string]any{}
	compNodeconfInfo    = ObjInfo{
		DefaultPrefix: "OSVC_COMP_NODECONF_",
		ExampleValue: []CompNodeconf{
			{
				Key:   "node.repopkg",
				Op:    "=",
				Value: "ftp://ftp.opensvc.com/opensvc",
			},
			{
				Key:   "node.repocomp",
				Op:    "=",
				Value: "ftp://ftp.opensvc.com/compliance",
			},
		},
		Description: `* Verify opensvc agent configuration parameter
`,
		FormDefinition: `Desc: |
  A rule to set a parameter in OpenSVC node.conf configuration file. Used by the 'nodeconf' compliance object.
Css: comp48
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: nodeconf
Inputs:
  -
    Id: key
    Label: Key
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The OpenSVC node.conf parameter to check.
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
      - "unset"
    Help: The comparison operator to use to check the parameter value.
  -
    Id: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Type: string or integer
    Help: The OpenSVC node.conf parameter value to check.
`,
	}
)

func init() {
	m["nodeconf"] = NewCompNodeConfs
}

func NewCompNodeConfs() interface{} {
	return &CompNodeconfs{
		Obj: NewObj(),
	}
}

func (t *CompNodeconfs) Add(s string) error {
	var data []CompNodeconf
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	for _, rule := range data {
		if rule.Key == "" {
			return fmt.Errorf("key is mandatory in dict : %s", s)
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "unset") {
			return fmt.Errorf("op is mandatory (and must be in =, >=, <=, unset) in dict : %s", s)
		}
		if rule.Value == nil {
			if rule.Op != "unset" {
				return fmt.Errorf("value is mandatory (except if operator is unset) in dict : %s", s)
			}
		} else {
			rule.Value = fmt.Sprint(rule.Value)
		}
		t.aggregateBlacklist(rule)
		t.Obj.Add(rule)
	}
	t.filterNodeConfUsingBlacklist()
	return nil
}

func (t *CompNodeconfs) aggregateBlacklist(rule CompNodeconf) {
	if _, ok := blacklistedNodeConf[rule.Key]; ok {
		return
	}
	objRule, ok := ruleNodeConf[rule.Key]
	if !ok {
		ruleNodeConf[rule.Key] = rule
		return
	}
	switch rule.Op {
	case "unset":
		if objRule.Op != "unset" {
			t.Errorf("conflict with the key %s: trying to unset and to compare a value at the same time the key is now blacklisted\n", rule.Key)
			blacklistedNodeConf[rule.Key] = nil
		}
	default:
		if objRule.Op == "unset" {
			t.Errorf("conflict with the key %s: trying to unset and to compare a value at the same time the key is now blacklisted\n", rule.Key)
			blacklistedNodeConf[rule.Key] = nil
		}
	}
}

func (t *CompNodeconfs) filterNodeConfUsingBlacklist() {
	newobj := NewCompNodeConfs().(*CompNodeconfs)
	for _, rule := range t.rules {
		rule := rule.(CompNodeconf)
		if _, ok := blacklistedNodeConf[rule.Key]; !ok {
			newobj.Obj.Add(rule)
		}
	}
	*t = *newobj
}

func (t CompNodeconfs) checkRule(rule CompNodeconf) ExitCode {
	n, err := object.NewNode()
	if err != nil {
		t.Errorf("error can't open a new node obj to check the rule\n")
		return ExitNok
	}
	currentVal := n.Config().Get(key.Parse(rule.Key))
	if currentVal == "" {
		if rule.Op == "unset" {
			t.VerboseInfof("the node key %s is unset and should be unset\n", rule.Key)
			return ExitOk
		}
		t.VerboseErrorf("the node key %s is unset and should be set\n", rule.Key)
		return ExitNok
	}
	if rule.Op == "unset" {
		t.VerboseErrorf("the node key %s is set and should be unset\n", rule.Key)
		return ExitNok
	}

	if n.Config().HasKeyMatchingOp(*keyop.Parse(rule.Key + rule.Op + rule.Value.(string))) {
		t.VerboseInfof("the rule for the node key %s , operator %s, value %s is respected\n", rule.Key, rule.Op, rule.Value.(string))
		return ExitOk
	}
	t.VerboseErrorf("the rule for the node key %s , operator %s, value %s is not respected\n", rule.Key, rule.Op, rule.Value.(string))
	return ExitNok
}

func (t CompNodeconfs) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompNodeconf)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompNodeconfs) fixRule(rule CompNodeconf) ExitCode {
	if t.checkRule(rule) == ExitOk {
		return ExitOk
	}
	n, err := object.NewNode(object.WithLogger(plog.NewLogger(zerolog.Nop())))
	if err != nil {
		t.Errorf("error can't open a new node obj to fix the rule\n")
		return ExitNok
	}
	if rule.Op == "unset" {
		if err := n.Config().Unset(key.Parse(rule.Key)); err != nil {
			t.Errorf("error when trying to unset for the rule %s\n", rule)
			return ExitNok
		}
		return ExitOk
	}
	if err := n.Config().Set(*keyop.Parse(rule.Key + "=" + rule.Value.(string))); err != nil {
		t.Errorf("error when trying to set the rule: %s\n", err)
		return ExitNok
	}
	return ExitOk
}

func (t CompNodeconfs) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompNodeconf)
		e = e.Merge(t.fixRule(rule))
	}
	return e
}

func (t CompNodeconfs) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompNodeconfs) Info() ObjInfo {
	return compNodeconfInfo
}
