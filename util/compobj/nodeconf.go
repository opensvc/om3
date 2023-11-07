package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/key"
	"strconv"
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
	omGet = func(node *object.Node, ctx context.Context, kw string) (interface{}, error) {
		return node.Get(ctx, kw)
	}

	compNodeconfInfo = ObjInfo{
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
			return fmt.Errorf("key is mandatory in dict : %s \n", s)
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "unset") {
			return fmt.Errorf("op is mandatory (and must be in =, >=, <=, unset) in dict : %s \n", s)
		}
		if rule.Value == nil {
			if rule.Op != "unset" {
				return fmt.Errorf("value is mandatory (except if operator is unset) in dict : %s \n", s)
			}
		}
		t.Obj.Add(rule)
	}
	return nil
}
func (t CompNodeconfs) checkRule(rule CompNodeconf) ExitCode {
	n, err := object.NewNode()
	if err != nil {
		t.Errorf("error when trying to create a newNode obj : %s\n", err)
		return ExitNok
	}
	ctx := context.Background()
	value, err := omGet(n, ctx, rule.Key)
	if err != nil {
		t.Errorf("error when trying to get the value of the key %s : %s\n", rule.Value, err)
		return ExitNok
	}

	if rule.Op == "unset" {
		if value.(string) == "" {
			t.VerboseInfof("the key %s is unset and should be unset --> ok \n", rule.Key)
			return ExitOk
		}
		t.VerboseInfof("the key %s should be unset unset and is not unset (value = %s) --> not ok\n", rule.Key, value.(string))
		return ExitNok
	}

	if value.(string) == "" {
		t.VerboseInfof("the key %s is unset and should not be unset --> not ok \n", rule.Key)
		return ExitNok
	}

	switch rule.Value.(type) {
	case float64:
		valuef, err := strconv.ParseFloat(value.(string), 64)
		if err != nil {
			t.Errorf("can't convert the value %s in float64", value)
			return ExitNok
		}
		if testOperatorFloat64(valuef, rule.Value.(float64), rule.Op) {
			t.VerboseInfof("%s = %f target: %f operator: %s --> ok\n", rule.Key, valuef, rule.Value, rule.Op)
			return ExitOk
		}
		t.VerboseInfof("%s = %f target: %f operator: %s --> not ok\n", rule.Key, valuef, rule.Value, rule.Op)
		return ExitNok
	case string:
		if rule.Op != "=" {
			t.VerboseInfof("only the unset and = operators are accepted for strings\n")
			return ExitNok
		}
		if testOperatorString(value.(string), rule.Value.(string), rule.Op) {
			t.VerboseInfof("%s = %s target: %s operator: %s --> ok\n", rule.Key, value.(string), rule.Value, rule.Op)
			return ExitOk
		}
		t.VerboseInfof("%s = %s target: %s operator: %s --> not ok\n", rule.Key, value.(string), rule.Value, rule.Op)
		return ExitNok
	default:
		t.Errorf("type of %s is not float64 or string\n", rule.Value)
		return ExitNok
	}

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
	n, err := object.NewNode()
	if err != nil {
		t.Errorf("error can't open the node object to use the unset and set commands :%s \n", err)
		return ExitNok
	}
	ctx := context.Background()
	if rule.Op == "unset" {
		if err := n.Unset(ctx, key.ParseStrings([]string{rule.Key})...); err != nil {
			t.Errorf("error can't use the om unset command : %s \n", err)
			return ExitNok
		}
		return ExitOk
	}
	var value string
	if _, ok := rule.Value.(float64); ok {
		value = strconv.FormatFloat(rule.Value.(float64), 'f', -1, 64)
	} else {
		value, ok = rule.Value.(string)
		if !ok {
			t.Errorf("can't convert the value %s into a string value", rule.Value)
			return ExitNok
		}
	}
	if err := n.Set(ctx, keyop.ParseOps([]string{rule.Key + "=" + value})...); err != nil {
		t.Errorf("error can't use the om set command : %s \n", err)
		return ExitNok
	}
	return ExitOk
}

func (t CompNodeconfs) Fix() ExitCode {
	t.SetVerbose(false)
	for _, i := range t.Rules() {
		rule := i.(CompNodeconf)
		if e := t.fixRule(rule); e == ExitNok {
			return ExitNok
		}
	}
	return ExitOk
}

func (t CompNodeconfs) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompNodeconfs) Info() ObjInfo {
	return compNodeconfInfo
}
