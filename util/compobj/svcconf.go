package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"encoding/json"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/key"
)

type (
	CompSvcconfs struct {
		*Obj
	}
	CompSvcconf struct {
		Key   string `json:"key"`
		Op    string `json:"op"`
		Value any    `json:"value"`
	}
)

var (
	svcRessourcesNames []string
	svcName            string
	compSvcconfInfo    = ObjInfo{
		DefaultPrefix: "OSVC_COMP_SVCCONF_",
		ExampleEnv: map[string]string{
			"OSVC_COMP_SERVICES_SVCNAME": "testsvc",
		},
		ExampleValue: []CompSvcconf{
			{
				Value: "fd5373b3d938",
				Key:   "container#1.run_image",
				Op:    "=",
			},
			{
				Value: "/bin/sh",
				Key:   "container#1.run_command",
				Op:    "=",
			},
			{
				Value: "/opt/%%ENV:SERVICES_SVCNAME%%",
				Key:   "DEFAULT.docker_data_dir",
				Op:    "=",
			},
			{
				Value: "no",
				Key:   "container(type=docker).disable",
				Op:    "=",
			},
			{
				Value: 123,
				Key:   "container(type=docker&&run_command=/bin/sh).newvar",
				Op:    "=",
			},
		},
		Description: `* Setup and verify parameters in a opensvc service configuration.
`,
		FormDefinition: `Desc: |
  A rule to set a parameter in OpenSVC <service>.conf configuration file. Used by the 'svcconf' compliance object.
Css: comp48
Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: svcconf
Inputs:
  -
    Id: key
    Label: Key
    DisplayModeLabel: key
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The OpenSVC <service>.conf parameter to check.
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
    Mandatory: Yes
    Type: string or integer
    Help: The OpenSVC <service>.conf parameter value to check.
`,
	}
)

func init() {
	m["svcconf"] = NewCompSvcConfs
}

func NewCompSvcConfs() interface{} {
	return &CompSvcconfs{
		Obj: NewObj(),
	}
}

func (t *CompSvcconfs) Add(s string) error {
	var data []CompSvcconf
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return err
	}
	var exists bool
	svcName, exists = os.LookupEnv("OSVC_COMP_SERVICES_SVCNAME")
	if !exists {
		return fmt.Errorf("the environment variable SERVICES_SVCNAME is not set in the os")
	}
	p, err := naming.ParsePath(svcName)
	if err != nil {
		return err
	}
	o, err := object.NewSvc(p)
	if err != nil {
		return fmt.Errorf("error can't create an configurer obj: %s", err)
	}
	svcRessourcesNames = o.Config().SectionStrings()
	svcRessourcesNames = t.addEnvInRessourcesNamesIfNotPresent(svcRessourcesNames)
	for _, rule := range data {
		if rule.Key == "" {
			return fmt.Errorf("key is mandatory in dict : %s", s)
		}
		if rule.Op == "" {
			rule.Op = "="
		}
		if !(rule.Op == "=" || rule.Op == ">=" || rule.Op == "<=" || rule.Op == "unset") {
			return fmt.Errorf("op must be in =, >=, <= in dict: %s", s)
		}
		if rule.Value == nil {
			if rule.Op == "unset" {
				rule.Value = "nil"
			} else {
				return fmt.Errorf("value is mandatory in dict : %s", s)
			}
		}
		rule.Value = fmt.Sprint(rule.Value)
		t.Obj.Add(rule)
	}
	return nil
}

func (t CompSvcconfs) addEnvInRessourcesNamesIfNotPresent(ressourcesNames []string) []string {
	for _, ressource := range ressourcesNames {
		if ressource == "env" {
			return ressourcesNames
		}
	}
	return append(ressourcesNames, "env")
}

func (t *CompSvcconfs) getKeyParts(rule CompSvcconf) (string, string, string) {
	reg1 := regexp.MustCompile(`(.*)\((.*)\)\.(.*)`)
	reg2 := regexp.MustCompile(`(.*)\.(.*)`)
	var section, filter, variable string
	match := reg1.FindStringSubmatch(rule.Key)
	if len(match) > 0 {
		section = match[1]
		filter = match[2]
		variable = match[3]
		return section, filter, variable
	}
	match = reg2.FindStringSubmatch(rule.Key)
	if len(match) > 0 {
		section = match[1]
		variable = match[2]
	}
	return section, filter, variable
}

func (t CompSvcconfs) checkRessourceName(resourceName string, ruleSection string) bool {
	return strings.HasPrefix(resourceName, ruleSection+"#") || resourceName == ruleSection
}

func (t CompSvcconfs) checkFilter(resourceName string, filter string) bool {
	if filter == "" {
		return true
	}
	o, err := object.NewConfigurer(naming.Parse(svcName))
	var op, leftFilter, rightFilter string
	if err != nil {
		t.Errorf("can't create an configurer obj: %s\n", err)
		return false
	}

	i := strings.Index(filter, "&&")
	if i != -1 {
		op = "and"
	}
	tmpi := strings.Index(filter, "||")
	if tmpi != -1 {
		op = "or"
		i = tmpi
	}
	if i != -1 {
		leftFilter = filter[:i]
		rightFilter = strings.TrimLeft(strings.TrimLeft(filter[i:], "&&"), "||")
	}
	switch op {
	case "and":
		return t.checkFilter(resourceName, leftFilter) && t.checkFilter(resourceName, rightFilter)
	case "or":
		return t.checkFilter(resourceName, leftFilter) || t.checkFilter(resourceName, rightFilter)
	default:
		return o.Config().HasKeyMatchingOp(*keyop.Parse(resourceName + "." + filter))
	}
}

func (t CompSvcconfs) checkValue(resourceName string, keyName string, value string, op string) bool {
	o, err := object.NewConfigurer(svcName)
	if err != nil {
		t.Errorf("can't create an configurer obj: %s\n", err)
		return false
	}
	if op == "unset" {
		return !o.Config().HasKey(key.New(resourceName, keyName))
	}
	return o.Config().HasKeyMatchingOp(*keyop.Parse(resourceName + "." + keyName + op + value))
}

func (t CompSvcconfs) checkSection(resourceName string, rule CompSvcconf) bool {
	o, err := object.NewConfigurer(svcName)
	if err != nil {
		t.Errorf("can't create an configurer obj: %s\n", err)
		return false
	}
	ruleSection, filter, keyName := t.getKeyParts(rule)
	if t.checkRessourceName(resourceName, ruleSection) && t.checkFilter(resourceName, filter) {
		isRuleRespected := t.checkValue(resourceName, keyName, rule.Value.(string), rule.Op)
		t.displayLogs(svcName, resourceName, o.Config().Get(key.Parse(rule.Key)), fmt.Sprintf("%s", rule.Value), rule.Op, isRuleRespected)
		return isRuleRespected
	}
	return true
}

func (t CompSvcconfs) displayLogs(svcName, resourceName, currentValue, ruleValue, operator string, isRuleRespected bool) {
	switch operator {
	case "=":
		if isRuleRespected {
			t.VerboseInfof("the resource %s of the svc %s is supposed to be equal to %s and is equal to %s\n", resourceName, svcName, ruleValue, currentValue)
			return
		}
		if currentValue == "" {
			t.VerboseErrorf("the resource %s of the svc %s is supposed to be equal to %s but is unset\n", resourceName, svcName, ruleValue)
			return
		}
		t.Errorf("the resource %s of the svc %s is supposed to be equal to %s but is equal to %s\n", resourceName, svcName, ruleValue, currentValue)
	case "<=":
		if isRuleRespected {
			t.VerboseInfof("the resource %s of the svc %s is equal to %s and should be less or equal to %s\n", resourceName, svcName, currentValue, ruleValue)
			return
		}
		if currentValue == "" {
			t.VerboseErrorf("the resource %s of the svc %s is unset and should be less or equal to %s\n", resourceName, svcName, ruleValue)
			return
		}
		t.VerboseErrorf("the resource %s of the svc %s is equal to %s and should be less or equal to %s\n", resourceName, svcName, currentValue, ruleValue)
	case ">=":
		if isRuleRespected {
			t.VerboseInfof("the resource %s of the svc %s is equal to %s and should be greater or equal to %s\n", resourceName, svcName, currentValue, ruleValue)
			return
		}
		if currentValue == "" {
			t.VerboseErrorf("the resource %s of the svc %s is unset and should be greater or equal to %s\n", resourceName, svcName, ruleValue)
			return
		}
		t.VerboseErrorf("the resource %s of the svc %s is equal to %s and should be greater or equal to %s\n", resourceName, svcName, currentValue, ruleValue)
	default:
		if isRuleRespected {
			t.VerboseInfof("the resource %s of the svc %s is unset and should be unset\n", resourceName, svcName)
			return
		}
		t.VerboseErrorf("the resource %s of the svc %s is equal to %s and should be unset\n", resourceName, svcName, currentValue)
	}
}

func (t CompSvcconfs) checkRule(rule CompSvcconf) ExitCode {
	e := ExitOk
	for _, resourceName := range svcRessourcesNames {
		if t.checkSection(resourceName, rule) {
			e = e.Merge(ExitOk)
			continue
		}
		e = e.Merge(ExitNok)
	}
	return e
}

func (t CompSvcconfs) Check() ExitCode {
	t.SetVerbose(true)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSvcconf)
		o := t.checkRule(rule)
		e = e.Merge(o)
	}
	return e
}

func (t CompSvcconfs) fixRule(rule CompSvcconf) ExitCode {
	e := ExitOk
	changement := false
	for _, resourceName := range svcRessourcesNames {
		if t.checkSection(resourceName, rule) {
			t.VerboseInfof("the resource %s respect the rule %s%s%s\n", resourceName, rule.Key, rule.Op, rule.Value)
			e = e.Merge(ExitOk)
			continue
		}
		t.VerboseErrorf("the resource %s does not respect the rule %s%s%s\n", resourceName, rule.Key, rule.Op, rule.Value)
		changement = true
		o, err := object.NewConfigurer(svcName)
		if err != nil {
			t.Errorf("error can't create a configurer obj: %s\n", err)
			return ExitNok
		}
		_, _, variable := t.getKeyParts(rule)
		if rule.Op == "unset" {
			kw := key.New(resourceName, variable)
			if err := o.Config().PrepareUnset(kw); err != nil {
				t.Errorf("unset %s: %s", kw, err)
				return e.Merge(ExitNok)
			}
		} else {
			kop := keyop.Parse(resourceName + "." + variable + "=" + rule.Value.(string))
			if err := o.Config().PrepareSet(*kop); err != nil {
				t.Errorf("set %s: %s", *kop, err)
				e = e.Merge(ExitNok)
				continue
			}
		}
		if err := o.Config().Commit(); err != nil {
			t.Errorf("%s", err)
			e = e.Merge(ExitNok)
		}
	}
	if changement {
		if rule.Op == "unset" {
			t.Infof("unset the key %s in the configuration of the service %s\n", rule.Key, svcName)
		} else {
			t.Infof("set the key %s to %s in the configuration of the service %s\n", rule.Key, rule.Value.(string), svcName)
		}
	}
	return e
}

func (t CompSvcconfs) Fix() ExitCode {
	t.SetVerbose(false)
	e := ExitOk
	for _, i := range t.Rules() {
		rule := i.(CompSvcconf)
		e = e.Merge(t.fixRule(rule))
	}
	return e
}

func (t CompSvcconfs) Fixable() ExitCode {
	return ExitNotApplicable
}

func (t CompSvcconfs) Info() ObjInfo {
	return compSvcconfInfo
}
